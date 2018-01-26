package cmdbox

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"github.com/verdverm/vermui"
	"github.com/verdverm/vermui/events"
)

const emptyMsg = "press 'Ctrl-<space>' to enter a command or '/path/to/something' to navigate"

type Command interface {
	CommandName() string
	CommandUsage() string
	CommandHelp() string

	CommandCallback(args []string, context map[string]interface{})
}

type DefaultCommand struct {
	Name  string
	Usage string
	Help  string

	Callback func(args []string, context map[string]interface{})
}

func (DC *DefaultCommand) CommandName() string {
	return DC.Name
}

func (DC *DefaultCommand) CommandHelp() string {
	return DC.Help
}

func (DC *DefaultCommand) CommandUsage() string {
	return DC.Usage
}

func (DC *DefaultCommand) CommandCallback(args []string, context map[string]interface{}) {
	DC.Callback(args, context)
}

type CmdBoxWidget struct {
	*tview.InputField

	commands map[string]Command

	curr    string   // current input (potentially partial)
	hIdx    int      // where we are in history
	history []string // command history
}

func New() *CmdBoxWidget {
	cb := &CmdBoxWidget{
		InputField: tview.NewInputField(),
		commands:   make(map[string]Command),
		history:    []string{"acct x", "acct p", "coin USDT XRP", "coin USDT BTC", "coin USDT ETH"},
	}

	cb.InputField.
		SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor).
		SetLabel(" ")

	cb.Mount(nil)

	return cb
}

func (CB *CmdBoxWidget) Id() string {
	return CB.InputField.Id()
}

func (CB *CmdBoxWidget) AddCommandCallback(command string, callback func([]string, map[string]interface{})) Command {
	c := &DefaultCommand{
		Name:     command,
		Usage:    command,
		Help:     "no help for " + command,
		Callback: callback,
	}
	CB.commands[c.CommandName()] = c
	return c
}

func (CB *CmdBoxWidget) AddCommand(command Command) {
	// go events.SendCustomEvent("/console/info", "adding command: "+command.CommandName())
	CB.commands[command.CommandName()] = command
}

func (CB *CmdBoxWidget) RemoveCommand(command Command) {
	delete(CB.commands, command.CommandName())
}

func (CB *CmdBoxWidget) Mount(context map[string]interface{}) error {
	vermui.AddWidgetHandler(CB, "/sys/key/C-space", func(e events.Event) {
		CB.curr = ""
		CB.hIdx = len(CB.history)
		CB.SetText("")
		CB.SetBorderColor(tcell.Color69)

		vermui.SetFocus(CB)
	})

	vermui.AddWidgetHandler(CB, "/user/error", func(e events.Event) {
		str := fmt.Sprintf("%v", e.Data.(*events.EventCustom).Data())
		CB.SetBorderColor(tcell.ColorRed)
		CB.SetFieldTextColor(tcell.ColorOrange)
		CB.SetText(str)

		vermui.Unfocus()
	})

	CB.SetFinishedFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			input := CB.GetText()
			input = strings.TrimSpace(input)
			if input != "" {
				flds := strings.Fields(input)
				CB.Submit(flds[0], flds[1:])
				CB.SetText("")
				CB.SetBorderColor(tcell.Color27)
				vermui.Unfocus()
			}
		case tcell.KeyEscape:
			CB.SetText("")
			CB.SetBorderColor(tcell.Color27)
			vermui.Unfocus()
		case tcell.KeyTab:
		case tcell.KeyBacktab:
		default:
			go events.SendCustomEvent("/console/warn", fmt.Sprintf("cmdbox (fin-???-key): %v", key))

		}

	})

	return nil
}
func (CB *CmdBoxWidget) Unmount() error {
	vermui.RemoveWidgetHandler(CB, "/sys/key/C-space")
	vermui.RemoveWidgetHandler(CB, "/user/error")

	return nil
}

func (CB *CmdBoxWidget) Submit(command string, args []string) {
	if len(command) == 0 {
		return
	}
	CB.history = append(CB.history, command+" "+strings.Join(args, " "))
	command = strings.ToLower(command)
	if command[:1] == "/" {
		go events.SendCustomEvent("/router/dispatch", command)
		return
	}
	cmd, ok := CB.commands[command]
	if !ok {
		// render for the user
		go events.SendCustomEvent("/user/error", fmt.Sprintf("unknown command %q", command))
		// log to console
		go events.SendCustomEvent("/console/warn", fmt.Sprintf("unknown command %q", command))
		return
	}

	go cmd.CommandCallback(args, nil)
}

// InputHandler returns the handler for this primitive.
func (CB *CmdBoxWidget) InputHandler() func(tcell.Event, func(tview.Primitive)) {
	return CB.WrapInputHandler(func(event tcell.Event, setFocus func(p tview.Primitive)) {
		handle := CB.InputField.InputHandler()
		switch evt := event.(type) {
		case *tcell.EventKey:

			dist := 1

			// Process key evt.
			switch key := evt.Key(); key {

			// Upwards, back in history
			case tcell.KeyHome:
				dist = len(CB.history)
				fallthrough
			case tcell.KeyPgUp:
				dist += 4
				fallthrough
			case tcell.KeyUp: // Regular character.
				if CB.hIdx == len(CB.history) {
					CB.curr = CB.GetText()
				}
				CB.hIdx -= dist
				if CB.hIdx < 0 {
					CB.hIdx = 0
				}
				CB.SetText(CB.history[CB.hIdx])

			// Downwards, more recent in history
			case tcell.KeyEnd:
				dist = len(CB.history)
				fallthrough
			case tcell.KeyPgDn:
				dist += 4
				fallthrough
			case tcell.KeyDown:
				CB.hIdx += dist
				if CB.hIdx > len(CB.history) {
					CB.hIdx = len(CB.history)
				}

				if CB.hIdx == len(CB.history) {
					CB.SetText(CB.curr)
					return
				}
				CB.SetText(CB.history[CB.hIdx])

			// Default is to pass through to InputField handler
			default:
				CB.hIdx = len(CB.history)
				handle(event, setFocus)

			}

		}
	})
}
