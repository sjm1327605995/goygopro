//go:build !cgo

package ocgcore

import "C"
import (
	"fmt"
	"github.com/ebitengine/purego"
	"os"
	"path/filepath"
	"unsafe"
)

type OCGApi struct {
	handle            uintptr
	rootPath          string
	scriptDirectory   string
	databaseFile      string
	buffer            []byte
	CreateDuel        func(seed int32) uintptr
	StartDuel         func(pduel uintptr, options int32)
	EndDuel           func(pduel uintptr)
	SetPlayerInfo     func(pduel uintptr, playerId, LP, startCount, drawCount int32)
	GetLogMessage     func(pduel uintptr, buf []byte)
	GetMessage        func(pduel uintptr, buf []byte) int32
	Process           func(pduel uintptr) uint32
	NewCard           func(pduel uintptr, code uint32, owner, playerid, location, sequence, position uint8)
	NewTagCard        func(pduel uintptr, code uint32, owner, location uint8)
	QueryCard         func(pduel uintptr, playerid, location, sequence uint8, queryFlag int32, buf []byte, useCache int32) int32
	QueryFieldCount   func(pduel uintptr, playerid, location uint8) int32
	QueryFieldCard    func(pduel uintptr, playerId, location uint8, queryFlag uint32, buf []byte, useCache int32) int32
	QueryFieldInfo    func(pduel uintptr, buf []byte) int32
	SetResponseI      func(pduel uintptr, value int32)
	SetResponseB      func(pduel uintptr, buf []byte)
	PreloadScript     func(pduel uintptr, script string, len int32) int32
	SetScriptReader   func(f uintptr)
	SetCardReader     func(f uintptr)
	SetMessageHandler func(f uintptr)

	_scriptReader   ScriptReader
	_cardReader     CardReader
	_messageHandler MessageHandler
}

// YGOCardData 是卡片数据的紧凑表示
type YGOCardData struct {
	Id         uint32
	Alias      uint32
	Setcode    int64
	Type       uint32
	Level      uint32
	Attribute  uint32
	Race       uint32
	Attack     int64
	Defense    int64
	LScale     uint32
	RScale     uint32
	LinkMarker uint32
}

// 声明类型
type (
	ScriptReader   func(scriptNamePtr *C.char, slen *C.int) *C.uchar
	CardReader     func(cardId uint32, card *CardData) uint
	MessageHandler func(pduel uintptr, msgType uint32) uint
)
type Option func(api *OCGApi)

func WithRootPath(path string) Option {
	return func(api *OCGApi) {
		api.rootPath = path
	}
}
func WithScriptDirectory(path string) Option {
	return func(api *OCGApi) {
		api.scriptDirectory = path
	}
}

func WithDatabaseFile(path string) Option {
	return func(api *OCGApi) {
		api.databaseFile = path
	}
}
func WithScriptReader(scriptReader ScriptReader) Option {
	return func(api *OCGApi) {
		api._scriptReader = scriptReader
	}
}
func WithCardReader(cardReader CardReader) Option {
	return func(api *OCGApi) {
		api._cardReader = cardReader
	}
}
func WithMessageHandler(messageHandler MessageHandler) Option {
	return func(api *OCGApi) {
		api._messageHandler = messageHandler
	}
}

var API *OCGApi

func registerFunctions(api *OCGApi, libc uintptr) {
	purego.RegisterLibFunc(&api.CreateDuel, libc, "create_duel")
	purego.RegisterLibFunc(&api.StartDuel, libc, "start_duel")
	purego.RegisterLibFunc(&api.EndDuel, libc, "end_duel")
	purego.RegisterLibFunc(&api.SetPlayerInfo, libc, "set_player_info")
	purego.RegisterLibFunc(&api.GetLogMessage, libc, "get_log_message")
	purego.RegisterLibFunc(&api.GetMessage, libc, "get_message")
	purego.RegisterLibFunc(&api.Process, libc, "process")
	purego.RegisterLibFunc(&api.NewCard, libc, "new_card")
	purego.RegisterLibFunc(&api.NewTagCard, libc, "new_tag_card")
	purego.RegisterLibFunc(&api.QueryCard, libc, "query_card")
	purego.RegisterLibFunc(&api.QueryFieldCount, libc, "query_field_count")
	purego.RegisterLibFunc(&api.QueryFieldCard, libc, "query_field_card")
	purego.RegisterLibFunc(&api.QueryFieldInfo, libc, "query_field_info")
	purego.RegisterLibFunc(&api.SetResponseI, libc, "set_responsei")
	purego.RegisterLibFunc(&api.SetResponseB, libc, "set_responseb")
	purego.RegisterLibFunc(&api.PreloadScript, libc, "preload_script")
	purego.RegisterLibFunc(&api.SetScriptReader, libc, "set_script_reader")
	purego.RegisterLibFunc(&api.SetCardReader, libc, "set_card_reader")
	purego.RegisterLibFunc(&api.SetMessageHandler, libc, "set_message_handler")
}

// Init 初始化OCGWrapper
func Init(opts ...Option) error {
	var err error
	// 设置默认路径
	ocgApi := &OCGApi{
		rootPath:        ".",
		scriptDirectory: "script",
		databaseFile:    "cards.cdb",
		buffer:          make([]byte, 128*1024), // 128 KiB

	}
	ocgApi._scriptReader = ocgApi.defaultScriptReader
	ocgApi._cardReader = ocgApi.defaultCardReader
	ocgApi._messageHandler = ocgApi.defaultOnMessageHandler
	API = ocgApi
	for _, opt := range opts {
		opt(API)
	}

	// 加载ocgcore动态库
	libPath := filepath.Join(ocgApi.rootPath, "ocgcore.dll")
	ocgApi.handle, err = openLibrary(libPath)
	if err != nil {
		return err
	}
	registerFunctions(ocgApi, ocgApi.handle)
	ocgApi.SetCallback()
	return nil
}
func (o *OCGApi) SetCallback() {
	scriptReaderPtr := purego.NewCallback(o._scriptReader)
	cardReaderPtr := purego.NewCallback(o._cardReader)
	messageHandlerPtr := purego.NewCallback(o._messageHandler)
	o.SetScriptReader(scriptReaderPtr)
	o.SetCardReader(cardReaderPtr)
	o.SetMessageHandler(messageHandlerPtr)
}
func (o *OCGApi) Dispose() {

}
func (o *OCGApi) defaultScriptReader(scriptNameC *C.char, slen *C.int) *C.uchar {
	*slen = 0
	scriptName := C.GoString(scriptNameC)
	fmt.Println("Loading script:", scriptName)
	scriptPath := filepath.Join(API.scriptDirectory, scriptName)
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Println("Error reading script file:", err)
		return (*C.uchar)(nil)
	}

	*slen = C.int(len(data))

	return (*C.uchar)(C.CBytes(data))
}
func (o *OCGApi) defaultCardReader(code uint32, pData *CardData) uint {

	return uint(code)
}
func (o *OCGApi) defaultOnMessageHandler(duelPtr uintptr, msgType uint32) uint {
	duelLock.Lock()
	defer duelLock.Unlock()
	duel, has := duels[duelPtr]
	if has {
		duel.OnMessage(msgType)
	}
	return 0
}

// GoString converts C string to Go string
func GoString(c uintptr) string {
	ptr := *(*unsafe.Pointer)(unsafe.Pointer(&c))
	if ptr == nil {
		return ""
	}
	var length int
	for {
		if *(*byte)(unsafe.Add(ptr, uintptr(length))) == '\x00' {
			break
		}
		length++
	}
	return string(unsafe.Slice((*byte)(ptr), length))
}
