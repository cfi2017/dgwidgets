package dgwidgets

import (
	"github.com/bwmarrin/discordgo"
)

// NextMessageCreateC returns a channel for the next MessageCreate event
func nextMessageCreateC(s *discordgo.Session) chan *discordgo.MessageCreate {
	out := make(chan *discordgo.MessageCreate)
	s.AddHandlerOnce(func(_ *discordgo.Session, e *discordgo.MessageCreate) {
		out <- e
	})
	return out
}

// NextMessageReactionAddC returns a channel for the next MessageReactionAdd event
func nextMessageReactionAddC(s *discordgo.Session) chan *discordgo.MessageReactionAdd {
	out := make(chan *discordgo.MessageReactionAdd)
	s.AddHandlerOnce(func(_ *discordgo.Session, e *discordgo.MessageReactionAdd) {
		out <- e
	})
	return out
}

// Creates a new channel that triggers for each reaction on the message that wasn't added by the bot (message author)
func reactionAddForMessage(s *discordgo.Session, message *discordgo.Message) (out chan *discordgo.MessageReactionAdd, cancel func()) {
	out = make(chan *discordgo.MessageReactionAdd)
	cancel = s.AddHandler(func(_ *discordgo.Session, e *discordgo.MessageReactionAdd) {
		if e.MessageID == message.ID && e.UserID != message.Author.ID {
			out <- e
		}
	})
	return
}

// EmbedsFromString splits a string into a slice of MessageEmbeds.
//     txt     : text to split
//     chunklen: How long the text in each embed should be
//               (if set to 0 or less, it defaults to 2048)
func EmbedsFromString(txt string, chunklen int) []*discordgo.MessageEmbed {
	if chunklen <= 0 {
		chunklen = 2048
	}

	embeds := []*discordgo.MessageEmbed{}
	for i := 0; i < int((float64(len(txt))/float64(chunklen))+0.5); i++ {
		start := i * chunklen
		end := start + chunklen
		if end > len(txt) {
			end = len(txt)
		}
		embeds = append(embeds, &discordgo.MessageEmbed{
			Description: txt[start:end],
		})
	}
	return embeds
}
