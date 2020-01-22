package dgwidgets

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	ErrAlreadyRunning   = errors.New("err: Widget already running")
	ErrIndexOutOfBounds = errors.New("err: Index is out of bounds")
	ErrNilMessage       = errors.New("err: Message is nil")
	ErrNilEmbed         = errors.New("err: embed is nil")
	ErrNotRunning       = errors.New("err: embed is not running")
	ctx                 = context.Background()
)

// WidgetHandler ...
type WidgetHandler func(*Widget, *discordgo.MessageReaction)

// Widget is a message embed with reactions for buttons.
// Accepts custom handlers for reactions.
type Widget struct {
	sync.RWMutex
	Embed     *discordgo.MessageEmbed
	Message   *discordgo.Message
	Ses       *discordgo.Session
	ChannelID string
	Timeout   time.Duration

	// Handlers binds emoji names to functions
	Handlers map[string]WidgetHandler
	// keys stores the handlers keys in the order they were added
	Keys []string

	// Delete reactions after they are added
	DeleteReactions bool
	// Delete message when closed or timed out
	DeleteOnTimeout bool
	// Only allow listed users to use reactions.
	UserWhitelist []string

	running bool
	cancel  func()
}

// NewWidget returns a pointer to a Widget object
//    ses      : discordgo session
//    channelID: channelID to spawn the widget on
func NewWidget(ses *discordgo.Session, channelID string, embed *discordgo.MessageEmbed) *Widget {
	return &Widget{
		ChannelID:       channelID,
		Ses:             ses,
		Keys:            []string{},
		Handlers:        map[string]WidgetHandler{},
		DeleteReactions: true,
		Embed:           embed,
	}
}

// isUserAllowed returns true if the user is allowed
// to use this widget.
func (w *Widget) isUserAllowed(userID string) bool {
	if w.UserWhitelist == nil || len(w.UserWhitelist) == 0 {
		return true
	}
	for _, user := range w.UserWhitelist {
		if user == userID {
			return true
		}
	}
	return false
}

func (w *Widget) Close() error {
	if !w.Running() {
		return ErrNotRunning
	}
	w.cancel()
	w.setRunning(false)
	return nil
}

func (w *Widget) Hook(Session *discordgo.Session, ChannelID, MessageID string) error {
	if w.Running() {
		return ErrAlreadyRunning
	}
	w.setRunning(true)
	w.Ses = Session
	msg, err := Session.ChannelMessage(ChannelID, MessageID)
	if err != nil {
		return err
	}
	w.Message = msg
	if len(msg.Embeds) == 0 {
		return ErrNilEmbed
	}
	w.Embed = msg.Embeds[0]

	// run
	return w.listen()

}

func (w *Widget) listen() error {

	wCtx, cancelW := context.WithTimeout(ctx, w.Timeout)
	w.cancel = cancelW
	reactions, cancelH := reactionAddForMessage(w.Ses, w.Message)
	for {
		select {
		case re := <-reactions:
			// Ignore reactions sent by bot
			reaction := re.MessageReaction
			if reaction.MessageID != w.Message.ID || w.Ses.State.User.ID == reaction.UserID {
				continue
			}

			if v, ok := w.Handlers[reaction.Emoji.Name]; ok {
				if w.isUserAllowed(reaction.UserID) {
					go v(w, reaction)
				}
			}

			if w.DeleteReactions {
				go func() {
					time.Sleep(time.Millisecond * 250)
					_ = w.Ses.MessageReactionRemove(reaction.ChannelID, reaction.MessageID, reaction.Emoji.Name, reaction.UserID)
				}()
			}
			break
		case <-wCtx.Done():
			// remove the event listener
			cancelH()
			err := wCtx.Err()
			if err != nil {
				if w.DeleteOnTimeout && wCtx.Err() == context.DeadlineExceeded {
					_ = w.Ses.ChannelMessageDelete(w.ChannelID, w.Message.ID)
				}
			}
			return err
		}
	}
}

// Spawn spawns the widget in channel w.ChannelID
func (w *Widget) Spawn() error {
	if w.Running() {
		return ErrAlreadyRunning
	}
	w.setRunning(true)
	if w.Embed == nil {
		return ErrNilEmbed
	}
	// Create initial message.
	msg, err := w.Ses.ChannelMessageSendEmbed(w.ChannelID, w.Embed)
	if err != nil {
		return err
	}
	w.Message = msg

	// Add reaction buttons
	for _, v := range w.Keys {
		_ = w.Ses.MessageReactionAdd(w.Message.ChannelID, w.Message.ID, v)
	}

	// run
	return w.listen()

}

// Handle adds a handler for the given emoji name
//    emojiName: The unicode value of the emoji
//    handler  : handler function to call when the emoji is clicked
//               func(*Widget, *discordgo.MessageReaction)
func (w *Widget) Handle(emojiName string, handler WidgetHandler) error {
	if _, ok := w.Handlers[emojiName]; !ok {
		w.Keys = append(w.Keys, emojiName)
		w.Handlers[emojiName] = handler
	}
	// if the widget is running, append the added emoji to the message.
	if w.Running() && w.Message != nil {
		return w.Ses.MessageReactionAdd(w.Message.ChannelID, w.Message.ID, emojiName)
	}
	return nil
}

// QueryInput querys the user with ID `id` for input
//    prompt : Question prompt
//    userID : UserID to get message from
//    timeout: How long to wait for the user's response
func (w *Widget) QueryInput(prompt string, userID string, timeout time.Duration) (*discordgo.Message, error) {
	msg, err := w.Ses.ChannelMessageSend(w.ChannelID, "<@"+userID+">,  "+prompt)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = w.Ses.ChannelMessageDelete(msg.ChannelID, msg.ID)
	}()

	timeoutChan := make(chan int)
	go func() {
		time.Sleep(timeout)
		timeoutChan <- 0
	}()

	for {
		select {
		case usermsg := <-nextMessageCreateC(w.Ses):
			if usermsg.Author.ID != userID {
				continue
			}
			_ = w.Ses.ChannelMessageDelete(usermsg.ChannelID, usermsg.ID)
			return usermsg.Message, nil
		case <-timeoutChan:
			return nil, errors.New("timed out")
		}
	}
}

// Running returns w.running
func (w *Widget) Running() bool {
	w.RLock()
	running := w.running
	w.RUnlock()
	return running
}

// UpdateEmbed updates the embed object and edits the original message
//    embed: New embed object to replace w.Embed
func (w *Widget) UpdateEmbed(embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	if w.Message == nil {
		return nil, ErrNilMessage
	}
	return w.Ses.ChannelMessageEditEmbed(w.ChannelID, w.Message.ID, embed)
}

func (w *Widget) setRunning(b bool) {
	w.Lock()
	w.running = b
	w.Unlock()
}
