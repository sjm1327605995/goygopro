package tenon

import (
	"fmt"
	"sort"
)

type ResponseSender interface {
	RespondIdleCmd(index, command int32)
	RespondBattleCmd(index, command int32)
	ResolveDesc(uint32) map[string]any
	RespondSelectPlace(player, location, sequence int32)
	RespondSelectPlaces(places []int32)
	RespondSelectCard(indices []int32)
	RespondSelectUnselect(index int32)
	SendResponseI(value int32)
}

type ActionOption struct {
	Label    string
	Kind     string
	Code     uint32
	Player   int
	Location int
	Sequence int
	Index    int
}

type DuelController struct {
	responses ResponseSender
	model     *DuelModel
}

func NewDuelController(responses ResponseSender, model *DuelModel) *DuelController {
	return &DuelController{responses: responses, model: model}
}

func (c *DuelController) ToBattle() bool {
	if c.model.Prompt.Mode == "idle" && c.model.Prompt.ToBP {
		c.responses.RespondIdleCmd(0, 6)
		c.model.consumePrompt()
		return true
	}
	return false
}

func (c *DuelController) ToMain2() bool {
	if c.model.Prompt.Mode == "battle" && c.model.Prompt.ToM2 {
		c.responses.RespondBattleCmd(0, 2)
		c.model.consumePrompt()
		return true
	}
	return false
}

func (c *DuelController) ToEnd() bool {
	switch c.model.Prompt.Mode {
	case "idle":
		if c.model.Prompt.ToEP {
			c.responses.RespondIdleCmd(0, 7)
			c.model.consumePrompt()
			return true
		}
	case "battle":
		if c.model.Prompt.ToEP {
			c.responses.RespondBattleCmd(0, 3)
			c.model.consumePrompt()
			return true
		}
	}
	return false
}

func (c *DuelController) ShuffleHand() bool {
	if c.model.Prompt.Mode == "idle" && c.model.Prompt.Shuffle {
		c.responses.RespondIdleCmd(0, 8)
		c.model.consumePrompt()
		return true
	}
	return false
}

func (c *DuelController) HandOptions(code uint32, sequence int) []ActionOption {
	if c.model.Prompt.Mode == "idle" {
		options := []ActionOption{}
		options = appendCommandOptions(options, "召唤", "summon", 0, c.model.Prompt.Summon, code, sequence, locationHand)
		options = appendCommandOptions(options, "特殊召唤", "spsummon", 1, c.model.Prompt.SPSummon, code, sequence, locationHand)
		options = appendCommandOptions(options, "盖放", "mset", 3, c.model.Prompt.MSet, code, sequence, locationHand)
		options = appendCommandOptions(options, "盖放", "sset", 4, c.model.Prompt.SSet, code, sequence, locationHand)
		return c.appendActivations(options, c.model.Prompt.Activate, code, sequence, locationHand)
	}
	if c.model.Prompt.Mode == "battle" {
		return c.appendActivations(nil, c.model.Prompt.Activate, code, sequence, locationHand)
	}
	return nil
}

func appendCommandOptions(options []ActionOption, label, kind string, command int, actions []CommandAction, code uint32, sequence, location int) []ActionOption {
	for _, action := range actions {
		if action.Code == code && action.Sequence == sequence && (action.Location == 0 || action.Location == location) {
			options = append(options, ActionOption{Label: label, Kind: kind, Code: code, Sequence: sequence, Index: action.Index, Location: location})
		}
	}
	return options
}

func (c *DuelController) appendActivations(options []ActionOption, actions []CommandAction, code uint32, sequence, location int) []ActionOption {
	matches := []CommandAction{}
	for _, action := range actions {
		if action.Code == code && action.Sequence == sequence && action.Location == location {
			matches = append(matches, action)
		}
	}
	for _, action := range matches {
		label := "发动"
		if len(matches) > 1 && action.Desc != 0 {
			if text, _ := c.responses.ResolveDesc(action.Desc)["text"].(string); text != "" {
				label = fmt.Sprintf("发动：%s", text)
			}
		}
		options = append(options, ActionOption{Label: label, Kind: "activate", Code: code, Sequence: sequence, Index: action.Index, Location: location})
	}
	return options
}

func (c *DuelController) FieldOptions(code uint32, seat, location, sequence int) []ActionOption {
	if seat != 0 {
		return nil
	}
	options := []ActionOption{}
	if c.model.Prompt.Mode == "idle" || c.model.Prompt.Mode == "battle" {
		options = c.appendActivations(options, c.model.Prompt.Activate, code, sequence, location)
	}
	if c.model.Prompt.Mode == "battle" {
		for _, action := range c.model.Prompt.Attack {
			if action.Code == code && action.Player == seat && action.Sequence == sequence {
				options = append(options, ActionOption{Label: "攻击", Kind: "attack", Code: code, Sequence: sequence, Index: action.Index, Location: location})
			}
		}
	}
	if c.model.Prompt.Mode == "idle" {
		for _, action := range c.model.Prompt.Repos {
			if action.Code == code && action.Player == seat && action.Sequence == sequence {
				options = append(options, ActionOption{Label: "表示形式变更", Kind: "repos", Code: code, Sequence: sequence, Index: action.Index, Location: location})
			}
		}
	}
	return options
}

func (c *DuelController) ChoosePlace(seat, location, sequence int) bool {
	placement := c.model.Placement
	if placement == nil {
		return false
	}
	zoneIndex := -1
	for index, zone := range placement.Zones {
		if zone.Seat == seat && zone.Location == location && zone.Sequence == sequence {
			zoneIndex = index
			break
		}
	}
	if zoneIndex < 0 {
		return false
	}
	for index, selected := range placement.Selected {
		if selected == zoneIndex {
			placement.Selected = append(placement.Selected[:index], placement.Selected[index+1:]...)
			c.updatePlaceMessage()
			c.model.changed()
			return true
		}
	}
	if len(placement.Selected) >= placement.Count {
		return false
	}
	placement.Selected = append(placement.Selected, zoneIndex)
	if len(placement.Selected) < placement.Count {
		c.updatePlaceMessage()
		c.model.changed()
		return true
	}

	selected := make([]PlaceZone, 0, len(placement.Selected))
	for _, index := range placement.Selected {
		selected = append(selected, placement.Zones[index])
	}
	sort.SliceStable(selected, func(i, j int) bool {
		rank := func(zone PlaceZone) int {
			rank := 2
			if zone.Player == placement.Player {
				rank = 0
			}
			if zone.Location != locationMZone {
				rank++
			}
			return rank
		}
		left, right := rank(selected[i]), rank(selected[j])
		if left != right {
			return left < right
		}
		return selected[i].Sequence < selected[j].Sequence
	})
	if len(selected) == 1 {
		zone := selected[0]
		c.responses.RespondSelectPlace(int32(zone.Player), int32(zone.Location), int32(zone.Sequence))
	} else {
		response := make([]int32, 0, len(selected)*3)
		for _, zone := range selected {
			response = append(response, int32(zone.Player), int32(zone.Location), int32(zone.Sequence))
		}
		c.responses.RespondSelectPlaces(response)
	}
	c.model.Placement = nil
	c.model.changed()
	return true
}

func (c *DuelController) CancelOrUndoPlace() bool {
	placement := c.model.Placement
	if placement == nil {
		return false
	}
	if len(placement.Selected) > 0 {
		placement.Selected = placement.Selected[:len(placement.Selected)-1]
		c.updatePlaceMessage()
		c.model.changed()
		return true
	}
	if !placement.Cancelable {
		return false
	}
	c.responses.RespondSelectPlace(int32(placement.Player), 0, 0)
	c.model.Placement = nil
	c.model.changed()
	return true
}

func (c *DuelController) updatePlaceMessage() {
	placement := c.model.Placement
	if placement == nil {
		return
	}
	if placement.Disfield {
		c.model.Message = "请选择要变成不能使用的区域"
	} else {
		c.model.Message = "请选择卡片的位置"
	}
	if placement.Count > 1 {
		c.model.Message = fmt.Sprintf("%s（已选 %d/%d）", c.model.Message, len(placement.Selected), placement.Count)
	}
}

func (c *DuelController) ChooseCard(code uint32, seat, location, sequence int) bool {
	selection := c.model.CardSelection
	if selection == nil {
		return false
	}
	choiceIndex := -1
	for index, choice := range selection.Choices {
		if choice.Code == code && choice.Seat == seat && choice.Location == location && choice.Sequence == sequence {
			choiceIndex = index
			break
		}
	}
	if choiceIndex < 0 {
		return false
	}
	for index, selected := range selection.Selected {
		if selected == choiceIndex {
			selection.Selected = append(selection.Selected[:index], selection.Selected[index+1:]...)
			c.model.updateCardSelectionMessage()
			c.model.changed()
			return true
		}
	}
	if len(selection.Selected) >= selection.Max {
		return false
	}
	selection.Selected = append(selection.Selected, choiceIndex)
	if selection.Kind == "unselect" || len(selection.Selected) == selection.Max {
		return c.FinishCardSelection()
	}
	c.model.updateCardSelectionMessage()
	c.model.changed()
	return true
}

func (c *DuelController) FinishCardSelection() bool {
	selection := c.model.CardSelection
	if selection == nil || len(selection.Selected) < selection.Min || len(selection.Selected) > selection.Max || (selection.Kind == "unselect" && len(selection.Selected) != 1) {
		return false
	}
	indices := make([]int32, len(selection.Selected))
	for index, selected := range selection.Selected {
		indices[index] = int32(selection.Choices[selected].ResponseIndex)
	}
	if selection.Kind == "unselect" {
		c.responses.RespondSelectUnselect(indices[0])
	} else {
		c.responses.RespondSelectCard(indices)
	}
	c.model.CardSelection = nil
	c.model.changed()
	return true
}

func (c *DuelController) CancelOrUndoCardSelection() bool {
	selection := c.model.CardSelection
	if selection == nil {
		return false
	}
	if len(selection.Selected) >= selection.Min && len(selection.Selected) <= selection.Max && (selection.Kind != "unselect" || len(selection.Selected) == 1) {
		return c.FinishCardSelection()
	}
	if len(selection.Selected) > 0 {
		selection.Selected = selection.Selected[:len(selection.Selected)-1]
		c.model.updateCardSelectionMessage()
		c.model.changed()
		return true
	}
	if !selection.Cancelable {
		return false
	}
	c.responses.SendResponseI(-1)
	c.model.CardSelection = nil
	c.model.changed()
	return true
}

func (c *DuelController) Execute(option ActionOption) bool {
	mode := c.model.Prompt.Mode
	if mode != "idle" && mode != "battle" {
		return false
	}
	commands := map[string]int{"summon": 0, "spsummon": 1, "repos": 2, "mset": 3, "sset": 4, "activate": 5, "attack": 1}
	command, ok := commands[option.Kind]
	if !ok || (mode == "idle" && option.Kind == "attack") || (mode == "battle" && option.Kind != "activate" && option.Kind != "attack") {
		return false
	}
	if mode == "battle" {
		if option.Kind == "activate" {
			command = 0
		}
		c.responses.RespondBattleCmd(int32(option.Index), int32(command))
	} else {
		c.responses.RespondIdleCmd(int32(option.Index), int32(command))
	}
	c.model.consumePrompt()
	return true
}
