package logs

import (
	"bytes"
	"strings"

	"github.com/gorilla/websocket"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

const disconnectMessage = -1

type logSender struct {
	marshalizer       marshal.Marshalizer
	conn              wsConn
	writer            *logWriter
	log               logger.Logger
	allowProfileApply bool
	lastProfile       logger.Profile
}

// NewLogSender returns a new component that is able to communicate with the log viewer application.
// After the correct handshake it will send all logs that come through the logger subsystem.
//
// allowProfileApply controls whether a client-supplied logger Profile (the first frame) may mutate
// the process-global logger. It must be true ONLY for authenticated connections: /log can be served
// unauthenticated, and letting an anonymous client apply a profile let a remote peer mute (*:NONE)
// or flood (*:TRACE) node logging process-wide (GHSA-9v8p-frvj-2pcm / KLC-2438). On a secured /log
// the operator is authenticated, so profile control (e.g. bumping verbosity to debug a live node)
// is restored as a trusted action and reverted on disconnect.
func NewLogSender(marshalizer marshal.Marshalizer, conn wsConn, log logger.Logger, allowProfileApply bool) (*logSender, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(log) {
		return nil, ErrNilLogger
	}
	if conn == nil {
		return nil, ErrNilWsConn
	}

	ls := &logSender{
		marshalizer:       marshalizer,
		log:               log,
		conn:              conn,
		allowProfileApply: allowProfileApply,
	}

	err := ls.registerLogWriter()
	if err != nil {
		return nil, err
	}

	return ls, nil
}

func (ls *logSender) registerLogWriter() error {
	w := NewLogWriter()
	formatter, err := logger.NewLogLineWrapperFormatter(ls.marshalizer)
	if err != nil {
		return err
	}

	err = logger.AddLogObserver(w, formatter)
	if err != nil {
		return err
	}

	ls.writer = w

	return nil
}

// StartSendingBlocking initializes the handshake by waiting for the first frame and after that
// will start sending logs information while monitoring the current connection. When profile
// application is allowed (authenticated client), a profile applied during the handshake is
// reverted once the client disconnects.
func (ls *logSender) StartSendingBlocking() {
	if ls.allowProfileApply {
		ls.lastProfile = logger.GetCurrentProfile()
	}

	defer func() {
		_ = ls.conn.Close()
		_ = ls.writer.Close()
		_ = logger.RemoveLogObserver(ls.writer)

		if ls.allowProfileApply {
			_ = ls.lastProfile.Apply()
			ls.log.Info("reverted log profile", "profile", ls.lastProfile.String())
		}
	}()

	err := ls.waitForProfile()
	if err != nil {
		ls.log.Error(err.Error())
		return
	}

	go ls.monitorConnection()
	ls.doSendContinuously()
}

// waitForProfile performs the /log handshake: it reads the first client frame. A client-provided
// profile is applied to the process-global logger ONLY when allowProfileApply is set (authenticated
// connection). On an unauthenticated /log the profile is parsed (to reject malformed handshakes) but
// never applied, so a remote peer cannot mute (*:NONE) or flood (*:TRACE) node logging process-wide
// (GHSA-9v8p-frvj-2pcm / KLC-2438).
func (ls *logSender) waitForProfile() error {
	_, message, err := ls.conn.ReadMessage()
	if err != nil {
		return err
	}

	if bytes.Equal(message, []byte(core.DefaultLogProfileIdentifier)) {
		return nil
	}

	profile, err := logger.UnmarshalProfile(message)
	if err != nil {
		return err
	}

	if !ls.allowProfileApply {
		ls.log.Info("ignoring client-provided log profile on unauthenticated /log", "profile", profile.String())
		return nil
	}

	ls.log.Info("websocket log profile received", "profile", profile.String())
	if err := profile.Apply(); err != nil {
		return err
	}

	logger.NotifyProfileChange()
	return nil
}

func (ls *logSender) monitorConnection() {
	var err error
	var mt int

	defer func() {
		_ = ls.writer.Close()
	}()

	for {
		mt, _, err = ls.conn.ReadMessage()
		ls.log.Trace("message type", "value", mt)
		if mt == websocket.CloseMessage || mt == disconnectMessage {
			return
		}
		if err != nil {
			return
		}
	}
}

func (ls *logSender) doSendContinuously() {
	for {
		shouldStop := ls.sendMessage()
		if shouldStop {
			return
		}
	}
}

func (ls *logSender) sendMessage() (shouldStop bool) {
	data, ok := ls.writer.ReadBlocking()
	if !ok {
		return true
	}

	err := ls.conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		isConnectionClosed := strings.Contains(err.Error(), "websocket: close sent")
		if !isConnectionClosed {
			ls.log.Error("web socket error", "error", err.Error())
		} else {
			ls.log.Info("web socket", "connection", "closed")
		}

		return true
	}

	return false
}
