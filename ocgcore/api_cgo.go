package ocgcore

/*
#cgo CFLAGS: -Iinclude
#cgo   LDFLAGS:   -L${SRCDIR}/../  -locgcore

#include "ocgapi.h"
*/
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"
)

type OCGApi struct {
	handle          uintptr
	rootPath        string
	scriptDirectory string
	databaseFile    string
	buffer          []byte
}

var (
	_scriptReader   ScriptReader
	_cardReader     CardReader
	_messageHandler MessageHandler
)

//export goScriptReader
func goScriptReader(scriptName *C.char, slen *C.int) *C.uchar {
	return _scriptReader(scriptName, slen)
}

//export goMessageHandler
func goMessageHandler(data C.longlong, size C.uint32_t) {
	_messageHandler(data, size)
}

//export goCardReader
func goCardReader(cardID C.uint32_t, data *C.card_data) C.uint32_t {
	card := _cardReader(uint32(cardID))
	fmt.Printf("goCardReader%+v\n", card)
	if card != nil {
		data.code = C.uint32_t(card.Code)
		data.alias = C.uint32_t(card.Alias)
		for i := 0; i < 16; i++ {
			data.setcode[i] = C.uint16_t(card.Setcode[i])
		}
		data._type = C.uint32_t(card.Type)
		data.level = C.uint32_t(card.Level)
		data.attribute = C.uint32_t(card.Attribute)
		data.race = C.uint32_t(card.Race)
		data.attack = C.int32_t(card.Attack)
		data.defense = C.int32_t(card.Defense)
		data.lscale = C.uint32_t(card.LScale)
		data.rscale = C.uint32_t(card.RScale)
		data.link_marker = C.uint32_t(card.LinkMarker)
		return C.uint32_t(card.Code)
	}
	return 0
}

// 声明类型
type (
	ScriptReader   func(scriptNamePtr *C.char, slen *C.int) *C.uchar
	CardReader     func(cardId uint32) *CardData
	MessageHandler func(data C.longlong, size C.uint32_t)
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
		_scriptReader = scriptReader
	}
}
func WithCardReader(cardReader CardReader) Option {
	return func(api *OCGApi) {
		_cardReader = cardReader
	}
}
func WithMessageHandler(messageHandler MessageHandler) Option {
	return func(api *OCGApi) {
		_messageHandler = messageHandler
	}
}

var API *OCGApi

// Init 初始化OCGWrapper
func Init(opts ...Option) error {
	// 设置默认路径
	ocgApi := &OCGApi{
		rootPath:        ".",
		scriptDirectory: "script",
		databaseFile:    "cards.cdb",
		buffer:          make([]byte, 128*1024), // 128 KiB

	}

	_scriptReader = ocgApi.defaultScriptReader
	_cardReader = ocgApi.defaultCardReader
	_messageHandler = ocgApi.defaultOnMessageHandler
	API = ocgApi
	for _, opt := range opts {
		opt(API)
	}

	C.set_script_reader(C.script_reader(C.goScriptReader))
	C.set_message_handler(C.message_handler(C.goMessageHandler))
	C.set_card_reader(C.card_reader(C.goCardReader))

	return nil
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
func (o *OCGApi) defaultCardReader(code uint32) *CardData {

	return nil
}
func (o *OCGApi) defaultOnMessageHandler(duelPtr C.longlong, msgType C.uint32_t) {
	duelLock.Lock()
	defer duelLock.Unlock()
	duel, has := duels[uintptr(duelPtr)]
	if has {
		duel.OnMessage(uint32(msgType))
	}
	return
}
func (o *OCGApi) CreateDuel(seed int32) uintptr {
	return uintptr(C.create_duel(C.int32_t(seed)))
}
func (o *OCGApi) StartDuel(pduel uintptr, options int32) {
	C.start_duel(C.longlong(pduel), C.int32_t(options))
}
func (o *OCGApi) EndDuel(pduel uintptr) {
	C.end_duel(C.longlong(pduel))
}
func (o *OCGApi) SetPlayerInfo(pduel uintptr, playerId, LP, startCount, drawCount int32) {
	C.set_player_info(C.longlong(pduel), C.int32_t(playerId), C.int32_t(LP), C.int32_t(startCount), C.int32_t(drawCount))
}
func (o *OCGApi) GetLogMessage(pduel uintptr, buf []byte) {
	C.get_log_message(C.longlong(pduel), (*C.uchar)(unsafe.Pointer(&buf[0])))
}
func (o *OCGApi) GetMessage(pduel uintptr, buff []byte) int32 {
	return int32(C.get_message(C.longlong(pduel), (*C.uchar)(unsafe.Pointer(&buff[0]))))
}
func (o *OCGApi) Process(pduel uintptr) uint32 {
	return uint32(C.process(C.longlong(pduel)))
}
func (o *OCGApi) NewCard(pduel uintptr, code uint32, owner, playerid, location, sequence, position uint8) {
	C.new_card(C.longlong(pduel), C.uint32_t(code), C.uint8_t(owner), C.uint8_t(playerid), C.uint8_t(location), C.uint8_t(sequence), C.uint8_t(position))
}
func (o *OCGApi) NewTagCard(pduel uintptr, code uint32, owner, location uint8) {

}
func (o *OCGApi) QueryCard(pduel uintptr, playerid, location, sequence uint8, queryFlag int32, buf []byte, useCache int32) int32 {
	return int32(C.query_card(C.longlong(pduel), C.uint8_t(playerid), C.uint8_t(location), C.uint8_t(sequence), C.int32_t(queryFlag), (*C.uchar)(unsafe.Pointer(&buf[0])), C.int32_t(useCache)))
}
func (o *OCGApi) QueryFieldCount(pduel uintptr, playerid, location uint8) int32 {
	return int32(C.query_field_count(C.longlong(pduel), C.uint8_t(playerid), C.uint8_t(location)))
}
func (o *OCGApi) QueryFieldCard(pduel uintptr, playerId, location uint8, queryFlag uint32, buf []byte, useCache int32) int32 {
	return int32(C.query_field_card(C.longlong(pduel), C.uint8_t(playerId), C.uint8_t(location), C.int32_t(queryFlag), (*C.uchar)(unsafe.Pointer(&buf[0])), C.int32_t(useCache)))
}
func (o *OCGApi) QueryFieldInfo(pduel uintptr, buf []byte) int32 {
	return int32(C.query_field_info(C.longlong(pduel), (*C.uchar)(unsafe.Pointer(&buf[0]))))
}
func (o *OCGApi) SetResponseI(pduel uintptr, value int32) {
	C.set_responsei(C.longlong(pduel), C.int32_t(value))
}
func (o *OCGApi) SetResponseB(pduel uintptr, buf []byte) {
	C.set_responseb(C.longlong(pduel), (*C.uchar)(unsafe.Pointer(&buf[0])))
}
func (o *OCGApi) PreloadScript(pduel uintptr, script []byte) int32 {
	return int32(C.preload_script(C.longlong(pduel), (*C.char)(unsafe.Pointer(&script[0])), C.int32_t(len(script))))
}
