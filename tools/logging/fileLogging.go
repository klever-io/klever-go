package logging

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go-logger/redirects"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/tools"
)

const minFileLifeSizeSpan = time.Second
const defaultFileSizeSpan = 1 * time.Minute
const defaultFileLifeSpan = time.Hour * 24
const backupTimeFormat = "2006-01-02T15-04-05"
const defaultMaxBackups = 10
const defaultFileSize = int64(100 * 1024 * 1024)

var log = logger.GetOrCreate("core/logging")

// logInfo is a convenience struct to return the filename and its embedded
// timestamp.
type logInfo struct {
	timestamp time.Time
	os.FileInfo
}

// fileLogging is able to rotate the log files
type fileLogging struct {
	chLifeSpanChanged chan time.Duration
	chSizeSpanChanged chan time.Duration
	mutFile           sync.Mutex
	currentFile       *os.File
	workingDir        string
	defaultLogsPath   string
	logFilePrefix     string
	cancelFunc        func()
	mutIsClosed       sync.Mutex
	isClosed          bool
	MaxBackups        int
	MaxFileSize       int64
}

// byFormatTime sorts by newest time formatted in the name.
type byFormatTime []logInfo

func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byFormatTime) Len() int {
	return len(b)
}

// NewFileLogging creates a file log watcher used to break the log file into multiple smaller files
func NewFileLogging(workingDir string, defaultLogsPath string, logFilePrefix string) (*fileLogging, error) {
	fl := &fileLogging{
		workingDir:        workingDir,
		defaultLogsPath:   defaultLogsPath,
		logFilePrefix:     logFilePrefix,
		chLifeSpanChanged: make(chan time.Duration),
		chSizeSpanChanged: make(chan time.Duration),
		isClosed:          false,
		MaxBackups:        defaultMaxBackups,
		MaxFileSize:       defaultFileSize,
	}
	fl.recreateLogFile()

	//we need this function as to call file.Close() when the code panics and the defer func associated
	//with the file pointer in the main func will never be reached
	runtime.SetFinalizer(fl, func(fileLogHandler *fileLogging) {
		_ = fileLogHandler.currentFile.Close()
	})

	ctx, cancelFunc := context.WithCancel(context.Background())
	go fl.autoRecreateFile(ctx)
	fl.cancelFunc = cancelFunc

	return fl, nil
}

func (fl *fileLogging) createFile() (*os.File, error) {
	logDirectory := filepath.Join(fl.workingDir, fl.defaultLogsPath)

	return tools.CreateFile(
		tools.ArgCreateFileArgument{
			Prefix:        fl.logFilePrefix,
			Directory:     logDirectory,
			FileExtension: "log",
		},
	)
}

func (fl *fileLogging) recreateLogFile() {
	err := fl.checkAndRemove()
	//If ErrReadingLogFileDirectory is because it has no older logs to remove
	if err != nil && !errors.Is(err, common.ErrReadingLogFileDirectory) {
		log.Error("error checking and removing older file", "error", err)
		return
	}

	newFile, err := fl.createFile()
	if err != nil {
		log.Error("error creating new log file", "error", err)
		return
	}

	fl.mutFile.Lock()
	defer fl.mutFile.Unlock()

	oldFile := fl.currentFile
	err = logger.AddLogObserver(newFile, &logger.PlainFormatter{})
	if err != nil {
		log.Error("error adding log observer", "error", err)
		return
	}

	errNotCritical := redirects.RedirectStderr(newFile)
	log.LogIfError(errNotCritical, "step", "redirecting std error")

	fl.currentFile = newFile

	if oldFile == nil {
		return
	}

	errNotCritical = oldFile.Close()
	log.LogIfError(errNotCritical, "step", "closing old log file")

	errNotCritical = logger.RemoveLogObserver(oldFile)
	log.LogIfError(errNotCritical, "step", "removing old log observer")
}

func (fl *fileLogging) checkAndRemove() error {
	files, err := fl.oldLogFiles()
	if err != nil {
		return err
	}

	if len(files) == fl.MaxBackups {
		errRemove := os.Remove(filepath.Join(fl.workingDir, fl.defaultLogsPath, files[len(files)-1].Name()))
		if err == nil && errRemove != nil {
			err = errRemove
		}
	}

	return err
}

func (fl *fileLogging) autoRecreateFile(ctx context.Context) {
	fileLifeSpan := defaultFileLifeSpan
	checkFileSizeSpan := defaultFileSizeSpan

	for {
		select {
		case <-ctx.Done():
			log.Debug("closing fileLogging.autoRecreateFile go routine")
			return
		case <-time.After(fileLifeSpan):
			fl.recreateLogFile()
		case <-time.After(checkFileSizeSpan):
			fs, err := fl.currentFile.Stat()
			if err != nil {
				log.Warn("error getting file size")
				continue
			}
			if fs.Size() >= fl.MaxFileSize {
				fl.recreateLogFile()
			}
		case checkFileSizeSpan = <-fl.chSizeSpanChanged:
			log.Debug("changed log file size span", "new value", checkFileSizeSpan)
		case fileLifeSpan = <-fl.chLifeSpanChanged:
			log.Debug("changed log file life span", "new value", fileLifeSpan)
		}
	}
}

// oldLogFiles returns the list of backup log files stored in the same
// directory as the current log file, sorted by ModTime
func (fl *fileLogging) oldLogFiles() ([]logInfo, error) {
	logDirectory := filepath.Join(fl.workingDir, fl.defaultLogsPath)

	files, err := os.ReadDir(logDirectory)
	if err != nil {
		return nil, common.ErrReadingLogFileDirectory
	}
	logFiles := []logInfo{}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if t, err := fl.timeFromName(f.Name(), fl.logFilePrefix+"-", ".log"); err == nil {
			info, _ := f.Info()
			logFiles = append(logFiles, logInfo{t, info})
			continue
		}
	}

	sort.Sort(byFormatTime(logFiles))

	return logFiles, nil
}

// timeFromName extracts the formatted time from the filename by stripping off
// the filename's prefix and extension. This prevents someone's filename from
// confusing time.parse.
func (fl *fileLogging) timeFromName(filename, prefix, ext string) (time.Time, error) {
	if !strings.HasPrefix(filename, prefix) {
		return time.Time{}, errors.New("mismatched prefix")
	}
	if !strings.HasSuffix(filename, ext) {
		return time.Time{}, errors.New("mismatched extension")
	}

	ts := filename[len(prefix) : len(filename)-len(ext)]

	return time.Parse(backupTimeFormat, ts)
}

func (fl *fileLogging) SetFileRotation(lifeSpanDuration time.Duration, checkSizeSpanDuration time.Duration, maxBackups int, maxFileSize int64) error {
	if lifeSpanDuration < minFileLifeSizeSpan {
		return fmt.Errorf("%w, provided %v", common.ErrInvalidLogFileMinLifeSpan, lifeSpanDuration)
	}

	if checkSizeSpanDuration < minFileLifeSizeSpan {
		return fmt.Errorf("%w, provided %v", common.ErrInvalidLogFileSizeMinCheckSpan, checkSizeSpanDuration)
	}

	fl.mutIsClosed.Lock()
	defer fl.mutIsClosed.Unlock()

	if fl.isClosed {
		return common.ErrFileLoggingProcessIsClosed
	}

	fl.MaxBackups = maxBackups
	fl.MaxFileSize = maxFileSize
	fl.chLifeSpanChanged <- lifeSpanDuration
	fl.chSizeSpanChanged <- checkSizeSpanDuration
	return nil
}

// ChangeFileLifeSpan changes the log file span
func (fl *fileLogging) ChangeFileLifeSpan(newDuration time.Duration) error {
	if newDuration < minFileLifeSizeSpan {
		return fmt.Errorf("%w, provided %v", common.ErrInvalidLogFileMinLifeSpan, newDuration)
	}

	fl.mutIsClosed.Lock()
	defer fl.mutIsClosed.Unlock()

	if fl.isClosed {
		return common.ErrFileLoggingProcessIsClosed
	}

	fl.chLifeSpanChanged <- newDuration
	return nil
}

// Close closes the file logging handler
func (fl *fileLogging) Close() error {
	fl.mutIsClosed.Lock()
	if fl.isClosed {
		fl.mutIsClosed.Unlock()
		return nil
	}

	fl.isClosed = true
	fl.mutIsClosed.Unlock()

	fl.mutFile.Lock()
	err := fl.currentFile.Close()
	fl.mutFile.Unlock()

	fl.cancelFunc()

	return err
}

// IsInterfaceNil returns true if there is no value under the interface
func (fl *fileLogging) IsInterfaceNil() bool {
	return fl == nil
}
