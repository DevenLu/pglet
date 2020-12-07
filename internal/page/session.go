package page

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/pglet/pglet/internal/page/command"
	"github.com/pglet/pglet/internal/utils"
)

const (
	// ZeroSession is ID of zero session
	ZeroSession string = ""
	// ControlAutoIDPrefix is a prefix for auto-generated control IDs
	ControlAutoIDPrefix = "_"
	// ControlIDSeparator is a symbol between parts of control ID
	ControlIDSeparator = ":"
	// PageID is a reserved page ID
	PageID = "page"
)

type commandHandler = func(*Session, *command.Command) (string, error)

var (
	commandHandlers = map[string]commandHandler{
		command.Add:     add,
		command.Addf:    add,
		command.Set:     set,
		command.Setf:    set,
		command.Append:  appendHandler,
		command.Appendf: appendHandler,
		command.Get:     get,
		command.Clean:   clean,
		command.Cleanf:  clean,
		command.Remove:  remove,
		command.Removef: remove,
	}
)

// Session represents an instance of a page.
type Session struct {
	sync.RWMutex
	Page          *Page               `json:"page"`
	ID            string              `json:"id"`
	Controls      map[string]*Control `json:"controls"`
	nextControlID int
	clients       map[*Client]bool
	clientsMutex  sync.RWMutex
}

type AddCommandBatchItem struct {
	Command *command.Command
	Control *Control
}

// NewSession creates a new instance of Page.
func NewSession(page *Page, id string) *Session {
	s := &Session{}
	s.Page = page
	s.ID = id
	s.Controls = make(map[string]*Control)
	s.AddControl(NewControl("page", "", PageID))
	s.clients = make(map[*Client]bool)
	return s
}

// ExecuteCommand executes command and returns the result
func (session *Session) ExecuteCommand(cmd *command.Command) (result string, err error) {
	session.Lock()
	defer session.Unlock()

	log.Printf("Execute command for page %s session %s: %+v\n",
		session.Page.Name, session.ID, cmd)

	commandHandler := commandHandlers[strings.ToLower(cmd.Name)]
	if commandHandler == nil {
		return "", fmt.Errorf("Unknown command: %s", cmd.Name)
	}

	return commandHandler(session, cmd)
}

func add(session *Session, cmd *command.Command) (result string, err error) {

	// parent ID
	topParentID := cmd.Attrs["to"]
	topParentAt := -1
	if ta, err := strconv.Atoi(cmd.Attrs["at"]); err == nil {
		topParentAt = ta
	}

	if topParentID == "" {
		topParentID = getPageID()
	}

	//log.Println("COMMAND:", utils.ToJSON(cmd))

	// "Add" commands to process
	batch := make([]*AddCommandBatchItem, 0)

	// top command
	indent := 0
	if len(cmd.Values) > 0 {
		// single command
		batch = append(batch, &AddCommandBatchItem{
			Command: cmd,
		})
		indent = 2
	}

	// sub-commands
	for _, line := range cmd.Lines {
		if utils.WhiteSpaceOnly(line) {
			continue
		}

		childCmd, err := command.Parse(line, false)
		if err != nil {
			return "", err
		}
		childCmd.Name = "add"
		childCmd.Indent += indent
		batch = append(batch, &AddCommandBatchItem{
			Command: childCmd,
		})
	}

	// list of control IDs
	ids := make([]string, 0)

	// list of controls to broadcast
	payload := &AddPageControlsPayload{
		Controls: make([]*Control, 0),
	}

	// process batch
	for i, batchItem := range batch {

		// first value must be control type
		if len(batchItem.Command.Values) == 0 {
			return "", errors.New("Control type is not specified")
		}

		controlType := batchItem.Command.Values[0]

		parentID := ""
		parentAt := -1

		// find nearest parentID
		for pi := i - 1; pi >= 0; pi-- {
			if batch[pi].Command.Indent < batchItem.Command.Indent {
				parentID = batch[pi].Control.ID()
				break
			}
		}

		// parent wasn't found - use the topmost one
		if parentID == "" {
			parentID = topParentID
			parentAt = topParentAt
		}

		// control ID
		id := batchItem.Command.Attrs["id"]
		if id == "" {
			id = session.NextControlID()
		} else {
			// generate unique ID
			parentIDs := getControlParentIDs(parentID)
			id = strings.Join(append(parentIDs, id), ControlIDSeparator)
		}

		batchItem.Control = NewControl(controlType, parentID, id)

		if parentAt != -1 {
			batchItem.Control.SetAttr("at", parentAt)
			topParentAt++
		}

		for k, v := range batchItem.Command.Attrs {
			if !IsSystemAttr(k) {
				batchItem.Control.SetAttr(k, v)
			}
		}

		session.AddControl(batchItem.Control)
		ids = append(ids, id)
		payload.Controls = append(payload.Controls, batchItem.Control)
	}

	//log.Println("CONTROLS:", utils.ToJSON(session.Controls))

	// broadcast new controls to all connected web clients
	session.broadcastCommandToWebClients(NewMessage(AddPageControlsAction, payload))
	return strings.Join(ids, " "), nil
}

func get(session *Session, cmd *command.Command) (result string, err error) {

	// command format must be:
	// get <control-id> <property>
	if len(cmd.Values) < 2 {
		return "", errors.New("'get' command should have control ID and property specified")
	}

	// control ID
	id := cmd.Values[0]

	ctrl, ok := session.Controls[id]
	if !ok {
		return "", fmt.Errorf("control with ID '%s' not found", id)
	}

	// control property
	prop := cmd.Values[1]

	v := ctrl.GetAttr(prop)

	if v == nil {
		return "", nil
	}

	return v.(string), nil
}

func set(session *Session, cmd *command.Command) (result string, err error) {

	batch := make([]*command.Command, 0)

	// top command
	if len(cmd.Values) > 0 {
		// single command
		batch = append(batch, cmd)
	}

	// sub-commands
	for _, line := range cmd.Lines {
		if utils.WhiteSpaceOnly(line) {
			continue
		}

		childCmd, err := command.Parse(line, false)
		if err != nil {
			return "", err
		}
		childCmd.Name = "set"
		batch = append(batch, childCmd)
	}

	payload := &UpdateControlPropsPayload{
		Props: make([]map[string]interface{}, 0, 0),
	}

	for _, batchCmd := range batch {
		// command format must be:
		// get <control-id> <property>
		if len(batchCmd.Values) < 1 {
			return "", errors.New("'set' command should have control ID specified")
		}

		// control ID
		id := batchCmd.Values[0]

		ctrl, ok := session.Controls[id]
		if !ok {
			return "", fmt.Errorf("control with ID '%s' not found", id)
		}

		props := make(map[string]interface{})
		props["i"] = id

		// set control properties, except system ones
		for n, v := range batchCmd.Attrs {
			if !IsSystemAttr(n) {
				ctrl.SetAttr(n, v)
				props[n] = v
			}
		}

		payload.Props = append(payload.Props, props)
	}

	// broadcast control updates to all connected web clients
	session.broadcastCommandToWebClients(NewMessage(UpdateControlPropsAction, payload))
	return "", nil
}

func appendHandler(session *Session, cmd *command.Command) (result string, err error) {

	batch := make([]*command.Command, 0)

	// top command
	if len(cmd.Values) > 0 {
		// single command
		batch = append(batch, cmd)
	}

	// sub-commands
	for _, line := range cmd.Lines {
		if utils.WhiteSpaceOnly(line) {
			continue
		}

		childCmd, err := command.Parse(line, false)
		if err != nil {
			return "", err
		}
		childCmd.Name = "append"
		batch = append(batch, childCmd)
	}

	payload := &AppendControlPropsPayload{
		Props: make([]map[string]string, 0, 0),
	}

	for _, batchCmd := range batch {
		// command format must be:
		// append control-id property=value property=value ...
		if len(batchCmd.Values) < 1 {
			return "", errors.New("'append' command should have control ID specified")
		}

		// control ID
		id := batchCmd.Values[0]

		ctrl, ok := session.Controls[id]
		if !ok {
			return "", fmt.Errorf("control with ID '%s' not found", id)
		}

		props := make(map[string]string)
		props["i"] = id

		// set control properties, except system ones
		for n, v := range batchCmd.Attrs {
			if !IsSystemAttr(n) {
				ctrl.AppendAttr(n, v)
				props[n] = v
			}
		}

		payload.Props = append(payload.Props, props)
	}

	// broadcast control updates to all connected web clients
	session.broadcastCommandToWebClients(NewMessage(AppendControlPropsAction, payload))
	return "", nil
}

func clean(session *Session, cmd *command.Command) (result string, err error) {

	// command format:
	//    clean [id_1] [id_2] ... [at=index]

	ids := make([]string, 0)
	if len(cmd.Values) == 0 {
		// clean page if no IDs specified
		ids = append(ids, PageID)
	} else {
		ids = append(ids, cmd.Values...)
	}

	at := -1
	if a, err := strconv.Atoi(cmd.Attrs["at"]); err == nil {
		at = a
	}

	if at != -1 && len(ids) > 1 {
		return "", errors.New("'at' cannot be specified with a list of IDs")
	}

	// control ID
	for i, id := range ids {
		ctrl, ok := session.Controls[id]
		if !ok {
			return "", fmt.Errorf("control with ID '%s' not found", id)
		}

		if at != -1 {
			childIDs := ctrl.GetChildrenIds()
			if at > len(childIDs)-1 {
				return "", fmt.Errorf("'at' is out of range")
			}

			ids[i] = childIDs[at]
			ctrl, _ = session.Controls[ids[i]]
		}

		session.cleanControl(ctrl)
	}

	// broadcast command to all connected web clients
	session.broadcastCommandToWebClients(NewMessage(CleanControlAction, &CleanControlPayload{
		IDs: ids,
	}))
	return "", nil
}

func remove(session *Session, cmd *command.Command) (result string, err error) {

	// command format:
	//    remove [id_1] [id_2] ... [at=index]

	at := -1
	if a, err := strconv.Atoi(cmd.Attrs["at"]); err == nil {
		at = a
	}

	ids := make([]string, 0)
	if len(cmd.Values) == 0 && at == -1 {
		return "", errors.New("'page' control cannot be removed")
	} else if len(cmd.Values) == 0 {
		ids = append(ids, PageID)
	} else {
		ids = append(ids, cmd.Values...)
	}

	if at != -1 && len(ids) > 1 {
		return "", errors.New("'at' cannot be specified with a list of IDs")
	}

	// control ID
	for i, id := range ids {
		ctrl, ok := session.Controls[id]
		if !ok {
			return "", fmt.Errorf("control with ID '%s' not found", id)
		}

		if at != -1 {
			childIDs := ctrl.GetChildrenIds()
			if at > len(childIDs)-1 {
				return "", fmt.Errorf("'at' is out of range")
			}

			ids[i] = childIDs[at]
			ctrl, _ = session.Controls[ids[i]]
		}

		session.deleteControl(ctrl)
	}

	// broadcast command to all connected web clients
	session.broadcastCommandToWebClients(NewMessage(RemoveControlAction, &RemoveControlPayload{
		IDs: ids,
	}))
	return "", nil
}

func (session *Session) UpdateControlProps(props []map[string]interface{}) {
	session.Lock()
	defer session.Unlock()

	for _, p := range props {
		id := p["i"].(string)
		if ctrl, ok := session.Controls[id]; ok {

			// patch control properties
			for n, v := range p {
				if !IsSystemAttr(n) {
					ctrl.SetAttr(n, v)
				}
			}
		}
	}
}

// NextControlID returns the next auto-generated control ID
func (session *Session) NextControlID() string {
	nextID := fmt.Sprintf("%s%d", ControlAutoIDPrefix, session.nextControlID)
	session.nextControlID++
	return nextID
}

// AddControl adds a control to a page
func (session *Session) AddControl(ctrl *Control) error {
	if _, exists := session.Controls[ctrl.ID()]; exists {
		return nil
	}
	session.Controls[ctrl.ID()] = ctrl

	// find parent
	parentID := ctrl.ParentID()
	if parentID != "" {
		parentctrl, ok := session.Controls[parentID]

		if !ok {
			return fmt.Errorf("parent control with id '%s' not found", parentID)
		}

		// update parent's childIds
		if at := ctrl.At(); at != -1 {
			parentctrl.InsertChildID(ctrl.ID(), at)
		} else {
			parentctrl.AddChildID(ctrl.ID())
		}
	}

	return nil
}

func getControlParentIDs(parentID string) []string {
	var result []string
	result = make([]string, 0)
	idParts := strings.Split(parentID, ControlIDSeparator)
	for _, idPart := range idParts {
		if !isAutoID(idPart) {
			result = append(result, idPart)
		}
	}
	return result
}

func (session *Session) cleanControl(ctrl *Control) {

	// delete all descendants
	for _, descID := range session.getAllDescendantIds(ctrl) {
		delete(session.Controls, descID)
	}

	// clean up children collection
	ctrl.RemoveChildren()
}

func (session *Session) deleteControl(ctrl *Control) {

	// delete all descendants
	for _, descID := range session.getAllDescendantIds(ctrl) {
		delete(session.Controls, descID)
	}

	// delete control itself
	delete(session.Controls, ctrl.ID())

	// remove control from parent's children collection
	session.Controls[ctrl.ParentID()].RemoveChild(ctrl.ID())
}

func (session *Session) getAllDescendantIds(ctrl *Control) []string {
	return session.getAllDescendantIdsRecursively(make([]string, 0, 0), ctrl.ID())
}

func (session *Session) getAllDescendantIdsRecursively(descendantIds []string, ID string) []string {
	ctrl := session.Controls[ID]
	childrenIds := ctrl.GetChildrenIds()
	result := append(descendantIds, childrenIds...)
	for _, childID := range childrenIds {
		result = append(result, session.getAllDescendantIdsRecursively(make([]string, 0, 0), childID)...)
	}
	return result
}

func getPageID() string {
	return PageID
}

func isAutoID(id string) bool {
	return id == PageID || strings.HasPrefix(id, ControlAutoIDPrefix)
}

func (session *Session) broadcastCommandToWebClients(msg *Message) {

	serializedMsg, _ := json.Marshal(msg)

	for c := range session.clients {
		if c.role == WebClient {
			c.send(serializedMsg)
		}
	}
}

func (session *Session) registerClient(client *Client) {
	session.clientsMutex.Lock()
	defer session.clientsMutex.Unlock()

	if _, ok := session.clients[client]; !ok {
		log.Printf("Registering %v client %s to %s:%s",
			client.role, client.id, session.Page.Name, session.ID)

		session.clients[client] = true
	}
}

func (session *Session) unregisterClient(client *Client) {
	session.clientsMutex.Lock()
	defer session.clientsMutex.Unlock()

	log.Printf("Unregistering %v client %s from session %s:%s",
		client.role, client.id, session.Page.Name, session.ID)

	delete(session.clients, client)
}
