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

func messageRemove(s *discordgo.Session, message *discordgo.Message) (out chan bool, cancel func()) {
	var cancelMessageDelete, cancelMessageDeleteBulk, cancelChannelDelete, cancelGuildDelete func()
	out = make(chan bool)
	cancel = func() {
		cancelMessageDelete()
		cancelMessageDeleteBulk()
		cancelChannelDelete()
		cancelGuildDelete()
	}
	cancelMessageDelete = s.AddHandler(func(_ *discordgo.Session, e *discordgo.MessageDelete) {
		if e.ID == message.ID {
			close(out)
			cancel()
		}
	})
	cancelMessageDeleteBulk = s.AddHandler(func(_ *discordgo.Session, e *discordgo.MessageDelete) {
		if e.ID == message.ID {
			close(out)
			cancel()
		}
	})
	cancelChannelDelete = s.AddHandler(func(_ *discordgo.Session, e *discordgo.ChannelDelete) {
		if e.ID == message.ChannelID {
			close(out)
			cancel()
		}
	})
	cancelGuildDelete = s.AddHandler(func(_ *discordgo.Session, e *discordgo.GuildDelete) {
		if e.ID == message.GuildID {
			close(out)
			cancel()
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

	var embeds = make([]*discordgo.MessageEmbed, 0)
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
