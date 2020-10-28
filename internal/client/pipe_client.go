package client

import (
	log "github.com/sirupsen/logrus"

	"github.com/pglet/pglet/internal/page"
	"github.com/pglet/pglet/internal/page/command"
	"github.com/pglet/pglet/internal/utils"
)

type PipeClient struct {
	id         string
	pageName   string
	sessionID  string
	pipe       pipe
	hostClient *HostClient
}

func NewPipeClient(pageName string, sessionID string, hc *HostClient) (*PipeClient, error) {
	id, _ := utils.GenerateRandomString(10)

	pipe, err := newNamedPipe(id)

	if err != nil {
		return nil, err
	}

	pc := &PipeClient{
		id:         id,
		pageName:   pageName,
		sessionID:  sessionID,
		pipe:       pipe,
		hostClient: hc,
	}

	return pc, nil
}

func (pc *PipeClient) CommandPipeName() string {
	return pc.pipe.getCommandPipeName()
}

func (pc *PipeClient) Start() error {

	go pc.commandLoop()

	return nil
}

func (pc *PipeClient) commandLoop() {
	log.Println("Starting command loop...")

	for {
		// read next command from pipeline
		cmdText := pc.pipe.nextCommand()

		// parse command
		cmd, err := command.Parse(cmdText)
		if err != nil {
			log.Fatalln(err)
		}

		log.Printf("Send command: %+v", cmd)

		if cmd.Name == command.Quit {
			pc.close()
			return
		}

		pc.hostClient.CallAndForget(page.PageCommandFromHostAction, &page.PageCommandRequestPayload{
			PageName:  pc.pageName,
			SessionID: pc.sessionID,
			Command:   *cmd,
		})

		// // parse response
		// payload := &page.PageCommandResponsePayload{}
		// err = json.Unmarshal(*rawResult, payload)

		// if err != nil {
		// 	log.Fatalln("Error parsing response from PageCommandFromHostAction:", err)
		// }

		// // save command results
		// result := payload.Result
		// if payload.Error != "" {
		// 	result = fmt.Sprintf("error %s", payload.Error)
		// }

		// pc.pipe.writeResult("aaa" + result)
		pc.pipe.writeResult("bbb")
	}
}

func (pc *PipeClient) emitEvent(evt string) {
	pc.pipe.emitEvent(evt)
}

func (pc *PipeClient) close() {
	log.Println("Closing pipe client...")

	pc.pipe.close()

	pc.hostClient.UnregisterPipeClient(pc)
}
