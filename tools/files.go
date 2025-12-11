package tools

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1" // #nosec G505: Blocklisted import sha1 hash is what is desired in this case
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"golang.org/x/crypto/pbkdf2"
)

var log = logger.GetOrCreate("tools")

// readAll reads from r until an error or EOF and returns the data it read
// from the internal buffer allocated with a specified capacity.
func readAll(r io.Reader, capacity int64) (b []byte, err error) {
	var buf bytes.Buffer
	// If the buffer overflows, we will get bytes.ErrTooLarge.
	// Return that as an error. Any other panic remains.
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
			err = panicErr
		} else {
			panic(e)
		}
	}()
	if int64(int(capacity)) == capacity {
		buf.Grow(int(capacity))
	}
	_, err = buf.ReadFrom(r)
	return buf.Bytes(), err
}

// ReadAll reads from r until an error or EOF and returns the data it read.
// A successful call returns err == nil, not err == EOF. Because ReadAll is
// defined to read from src until EOF, it does not treat an EOF from Read
// as an error to be reported.
func ReadAll(r io.Reader) ([]byte, error) {
	return readAll(r, bytes.MinRead)
}

// ReadFile reads the file named by filename and returns the contents.
// A successful call returns err == nil, not err == EOF. Because ReadFile
// reads the whole file, it does not treat an EOF from Read as an error
// to be reported.
func ReadFile(filename string) ([]byte, error) {
	cleanFile := filepath.Clean(filename)
	f, err := os.Open(cleanFile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	// It's a good but not certain bet that FileInfo will tell us exactly how much to
	// read, so let's try it but be prepared for the answer to be wrong.
	var n int64 = bytes.MinRead

	if fi, err := f.Stat(); err == nil {
		// As initial capacity for readAll, use Size + a little extra in case Size
		// is zero, and to avoid another allocation after Read has filled the
		// buffer. The readAll call will read into its allocated internal buffer
		// cheaply. If the size was wrong, we'll either waste some space off the end
		// or reallocate as needed, but in the overwhelmingly common case we'll get
		// it just right.
		if size := fi.Size() + bytes.MinRead; size > n {
			n = size
		}
	}
	return readAll(f, n)
}

// WriteFile writes data to a file named by filename.
// If the file does not exist, WriteFile creates it with permissions perm;
// otherwise WriteFile truncates it before writing.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	cleanFile := filepath.Clean(filename)
	f, err := os.OpenFile(cleanFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm) // #nosec G304: desired file permissions
	if err != nil {
		return err
	}
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

// ReadDir reads the directory named by dirname and returns
// a list of directory entries sorted by filename.
func ReadDir(dirname string) ([]os.FileInfo, error) {
	f, err := os.Open(filepath.Clean(dirname))
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}

	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list, nil
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

// NopCloser returns a ReadCloser with a no-op Close method wrapping
// the provided Reader r.
func NopCloser(r io.Reader) io.ReadCloser {
	return nopCloser{r}
}

type devNull int

// devNull implements ReaderFrom as an optimization so io.Copy to
// io.Discard can avoid doing unnecessary work.
var _ io.ReaderFrom = devNull(0)

func (devNull) Write(p []byte) (int, error) {
	return len(p), nil
}

func (devNull) WriteString(s string) (int, error) {
	return len(s), nil
}

var blackHolePool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 8192)
		return &b
	},
}

func (devNull) ReadFrom(r io.Reader) (n int64, err error) {
	bufp := blackHolePool.Get().(*[]byte)
	readSize := 0
	for {
		readSize, err = r.Read(*bufp)
		n += int64(readSize)
		if err != nil {
			blackHolePool.Put(bufp)
			if err == io.EOF {
				return n, nil
			}
			return
		}
	}
}

// Discard is an io.Writer on which all Write calls succeed
// without doing anything.
var Discard io.Writer = devNull(0)

// ArgCreateFileArgument will hold the arguments for a new file creation method call
type ArgCreateFileArgument struct {
	Directory     string
	Prefix        string
	FileExtension string
}

// CreateFile opens or creates a file relative to the default path
func CreateFile(arg ArgCreateFileArgument) (*os.File, error) {
	absPath, err := filepath.Abs(arg.Directory)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(absPath, common.DefaultDirPermission)
	if err != nil {
		return nil, err
	}

	fileName := time.Now().Format("2006-01-02T15-04-05")
	if arg.Prefix != "" {
		fileName = arg.Prefix + "-" + fileName
	}
	fileName = fmt.Sprintf("%s.%s", fileName, arg.FileExtension)

	return os.OpenFile(
		filepath.Clean(filepath.Join(absPath, fileName)),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		common.FileModeUserReadWrite)
}

// OpenFile method opens the file from given path - does not close the file
func OpenFile(relativePath string) (*os.File, error) {
	path, err := filepath.Abs(relativePath)
	if err != nil {
		log.Warn("cannot create absolute path for the provided file", "error", err.Error())
		return nil, err
	}
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	return f, nil
}

// LoadSkPkFromPemFile loads the secret key and existing public key bytes stored in the file
func LoadSkPkFromPemFile(relativePath string, skIndex int, pwd string) ([]byte, string, error) {
	if skIndex < 0 {
		return nil, "", ErrInvalidIndex
	}

	file, err := OpenFile(relativePath)
	if err != nil {
		return nil, "", err
	}

	defer func() {
		cerr := file.Close()
		log.LogIfError(cerr)
	}()

	buff, err := io.ReadAll(file)
	if err != nil {
		return nil, "", fmt.Errorf("%w while reading %s file", err, relativePath)
	}
	if len(buff) == 0 {
		return nil, "", fmt.Errorf("%w while reading %s file", ErrEmptyFile, relativePath)
	}

	var blkRecovered *pem.Block

	for i := 0; i <= skIndex; i++ {
		if len(buff) == 0 {
			//less private keys present in the file than required
			return nil, "", fmt.Errorf("%w while reading %s file, invalid index %d", ErrInvalidIndex, relativePath, i)
		}

		blkRecovered, buff = pem.Decode(buff)
		if blkRecovered == nil {
			return nil, "", fmt.Errorf("%w while reading %s file, error decoding", ErrPemFileIsInvalid, relativePath)
		}

		if IsEncryptedPEMBlock(blkRecovered) {
			if len(pwd) == 0 {
				return nil, "", errors.New("encrypted key, must provide password")
			}
			blkRecovered, err = DecryptPEMBlock(blkRecovered, pwd)
			if err != nil {
				return nil, "", fmt.Errorf("failed PEM decryption: %w", err)
			}
		}
	}

	if blkRecovered == nil {
		return nil, "", ErrNilPemBLock
	}

	blockType := blkRecovered.Type
	header := "PRIVATE KEY for "
	if strings.Index(blockType, header) != 0 {
		return nil, "", fmt.Errorf("%w missing '%s' in block type", ErrPemFileIsInvalid, header)
	}

	blockTypeString := blockType[len(header):]

	return blkRecovered.Bytes, blockTypeString, nil
}

// LoadJSONFile method to open and decode json file
func LoadJSONFile(dest interface{}, relativePath string) error {
	f, err := OpenFile(relativePath)
	if err != nil {
		return err
	}

	defer func() {
		err = f.Close()
		if err != nil {
			log.Warn("cannot close file", "error", err.Error())
		}
	}()

	return json.NewDecoder(f).Decode(dest)
}

// IsEncryptedPEMBlock returns whether the PEM block is password encrypted
// according to RFC 1423.
func IsEncryptedPEMBlock(b *pem.Block) bool {
	_, ok := b.Headers["DEK-Info"]
	return ok
}

// DecryptPEMBlock takes a PEM block encrypted according to RFC 1423 and the
// password used to encrypt it and returns a slice of decrypted DER encoded
// bytes. It inspects the DEK-Info header to determine the algorithm used for
// decryption. If no DEK-Info header is present, an error is returned. If an
// incorrect password is detected an IncorrectPasswordError is returned.
func DecryptPEMBlock(b *pem.Block, pwd string) (*pem.Block, error) {
	dek, ok := b.Headers["DEK-Info"]
	if !ok {
		return nil, errors.New("x509: no DEK-Info header in block")
	}

	mode, _, ok := strings.Cut(dek, ",")
	if !ok {
		return nil, errors.New("x509: malformed DEK-Info header")
	}

	if mode != "AES-GCM" {
		return nil, errors.New("invalid encryption mode")
	}

	password := getEncryptionKey(pwd)

	block, err := aes.NewCipher(password)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if b.Bytes == nil || len(b.Bytes) < nonceSize {
		return nil, fmt.Errorf("invalid data size")
	}
	nonce, ciphertext := b.Bytes[:nonceSize], b.Bytes[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil) // #nosec G407 false positive
	if err != nil {
		return nil, err
	}

	return &pem.Block{
		Type:  b.Type,
		Bytes: plaintext,
	}, nil

}

// EncryptPEMBlock returns a PEM block of the specified type holding the
// given DER encoded data encrypted with GCM algorithm and
// password according to RFC 1423.
func EncryptPEMBlock(blockType string, data []byte, pwd string) (*pem.Block, error) {
	password := getEncryptionKey(pwd)

	block, _ := aes.NewCipher(password)
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	// append nonce to seal package
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return &pem.Block{
		Type: blockType,
		Headers: map[string]string{
			"Proc-Type": "4,ENCRYPTED",
			"DEK-Info":  "AES-GCM," + hex.EncodeToString(nonce),
		},
		Bytes: ciphertext,
	}, nil
}

// getEncryptionKey based for pin
func getEncryptionKey(pin string) []byte {
	idBytes := StringHash("kleverchain")
	pwdBytes := StringHash(pin)

	return PBKDFPass(pwdBytes, idBytes)
}

// StringHash computes SHA256 from string
func StringHash(text string) []byte {
	h := sha256.New()
	h.Write([]byte(text))
	return h.Sum(nil)
}

// PBKDFPass from password and salt
func PBKDFPass(password, salt []byte) []byte {
	return pbkdf2.Key(password, salt, 4096, 32, sha1.New)
}

func isValidFileName(fileName string) bool {
	validFileName := regexp.MustCompile(`^[a-zA-Z0-9_\-/\\]+(\.[a-zA-Z0-9]+)*$`)
	return validFileName.MatchString(fileName)
}

func getAbsolutePath(fileName string) (string, error) {
	if !isValidFileName(fileName) {
		return "", ErrInvalidFileName
	}

	absPath, err := filepath.Abs(fileName)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

// SanitizePath removes any leading slashes from the path
func SanitizePath(path string) (string, error) {
	return getAbsolutePath(path)
}
