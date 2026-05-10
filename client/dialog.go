package client

// DialogType represents the current active dialog/popup
type DialogType int

const (
	DialogNone DialogType = iota
	DialogYesNo
	DialogOption
	DialogCardSelect
	DialogChain
	DialogPlace
	DialogPosition
	DialogTribute
	DialogCounter
	DialogSum
	DialogDisfield
	DialogSort
	DialogUnselect
	DialogBattleCmd
	DialogIdleCmd
	DialogCmdSelect
	DialogRace
	DialogAttrib
	DialogCard
)

// DialogState holds the state for all possible dialogs.
// Only one dialog is active at a time.
type DialogState struct {
	Type     DialogType
	Title    string
	Visible  bool

	// YesNo
	YesNoDesc int32

	// Option
	Options []int32
	// OnOptionSelected is called instead of the default SetResponseI when non-nil.
	OnOptionSelected func(index int)

	// CardSelect
	SelectCancelable bool
	SelectMin        int
	SelectMax        int
	SelectCards      []*ClientCard
	SelectHint       string

	// Place
	SelectableField uint32
	SelectedField   uint32

	// CmdSelect
	CmdSelectOptions []string
	CmdSelectResp    []int32

	// Position
	PositionCode     uint32
	PositionOptions  uint8

	// Sort
	SortCards []*ClientCard
	SortOrder []int
	SortCur   int

	// Announce
	AnnounceRaceAvail     uint32
	AnnounceAttribAvail   uint32
	AnnounceCardOpcodes   []uint32
}

func NewDialogState() *DialogState {
	return &DialogState{Type: DialogNone}
}

func (d *DialogState) ShowYesNo(desc int32) {
	d.Type = DialogYesNo
	d.YesNoDesc = desc
	d.Title = "Confirm"
	d.Visible = true
}

func (d *DialogState) ShowOption(options []int32) {
	d.Type = DialogOption
	d.Options = options
	d.Title = "Select Option"
	d.Visible = true
}

func (d *DialogState) ShowCardSelect(cards []*ClientCard, min, max int, cancelable bool, hint string) {
	d.Type = DialogCardSelect
	d.SelectCards = cards
	d.SelectMin = min
	d.SelectMax = max
	d.SelectCancelable = cancelable
	d.SelectHint = hint
	d.Title = hint
	d.Visible = true
}

func (d *DialogState) ShowCmdSelect(options []string, resp []int32) {
	d.Type = DialogCmdSelect
	d.CmdSelectOptions = options
	d.CmdSelectResp = resp
	d.Title = "Select Command"
	d.Visible = true
}

func (d *DialogState) ShowPosition(code uint32, positions uint8) {
	d.Type = DialogPosition
	d.PositionCode = code
	d.PositionOptions = positions
	d.Title = "Select Position"
	d.Visible = true
}

func (d *DialogState) ShowSort(cards []*ClientCard) {
	d.Type = DialogSort
	d.SortCards = cards
	d.SortOrder = make([]int, len(cards))
	d.SortCur = 1
	d.Title = "Sort Cards"
	d.Visible = true
}

func (d *DialogState) ShowAnnounceRace(avail uint32) {
	d.Type = DialogRace
	d.AnnounceRaceAvail = avail
	d.Title = "Select Race"
	d.Visible = true
}

func (d *DialogState) ShowAnnounceAttrib(avail uint32) {
	d.Type = DialogAttrib
	d.AnnounceAttribAvail = avail
	d.Title = "Select Attribute"
	d.Visible = true
}

func (d *DialogState) ShowAnnounceCard(opcodes []uint32) {
	d.Type = DialogCard
	d.AnnounceCardOpcodes = opcodes
	d.Title = "Select Card"
	d.Visible = true
}

func (d *DialogState) Hide() {
	d.Type = DialogNone
	d.Visible = false
	d.Options = nil
	d.OnOptionSelected = nil
	d.SelectCards = nil
	d.CmdSelectOptions = nil
	d.CmdSelectResp = nil
}
