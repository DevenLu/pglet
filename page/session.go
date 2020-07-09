package page

import (
	"fmt"
	"log"
	"strconv"
	"sync"
)

const (
	ZeroSession string = ""
)

// Session represents an instance of a page.
type Session struct {
	sync.RWMutex
	Page          *Page              `json:"page"`
	ID            string             `json:"id"`
	Controls      map[string]Control `json:"controls"`
	nextControlID int
	clients       map[*Client]bool
	clientsMutex  sync.RWMutex
}

// NewSession creates a new instance of Page.
func NewSession(page *Page, id string) *Session {
	s := &Session{}
	s.Page = page
	s.ID = id
	s.Controls = make(map[string]Control)
	s.AddControl(NewControl("Page", "", s.NextControlID()))
	s.clients = make(map[*Client]bool)
	return s
}

func (session *Session) ExecuteCommand(command string) (result string, err error) {
	result = "a\nb\n" + command
	return
}

// NextControlID returns the next auto-generated control ID
func (session *Session) NextControlID() string {
	session.Lock()
	defer session.Unlock()
	nextID := strconv.Itoa(session.nextControlID)
	session.nextControlID++
	return nextID
}

// AddControl adds a control to a page
func (session *Session) AddControl(ctl Control) error {
	// find parent
	parentID := ctl.ParentID()
	if parentID != "" {
		session.RLock()
		parentCtl, ok := session.Controls[parentID]
		session.RUnlock()

		if !ok {
			return fmt.Errorf("parent control with id '%s' not found", parentID)
		}

		// update parent's childIds
		session.Lock()
		parentCtl.AddChildID(ctl.ID())
		session.Unlock()
	}

	session.Lock()
	defer session.Unlock()
	session.Controls[ctl.ID()] = ctl
	return nil
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

	log.Printf("Unregistering %v client %s from %s:%s",
		client.role, client.id, session.Page.Name, session.ID)

	delete(session.clients, client)
}
