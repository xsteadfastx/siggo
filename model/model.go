package model

import (
	"fmt"
	"log"

	"github.com/derricw/siggo/signal"
)

var DeliveryStatus map[bool]string = map[bool]string{
	true:  "<",
	false: "?",
}

var ReadStatus map[bool]string = map[bool]string{
	true:  ">",
	false: "?",
}

type Config struct {
	UserName   string
	UserNumber string
}

type Contact struct {
	Number string
	Name   string
}

type Message struct {
	Content     string
	From        string
	Timestamp   int64
	IsDelivered bool
	IsRead      bool
}

func (m *Message) String() string {
	return fmt.Sprintf("%d|%s%s %s: %s\n",
		m.Timestamp,
		DeliveryStatus[m.IsDelivered],
		ReadStatus[m.IsRead],
		m.From,
		m.Content,
	)
}

type Conversation struct {
	Contact       *Contact
	Messages      map[int64]*Message
	MessageOrder  []int64
	HasNewMessage bool
}

func (c *Conversation) String() string {
	out := ""
	for _, k := range c.MessageOrder {
		out += c.Messages[k].String()
	}
	return out
}

func (c *Conversation) AddMessage(message *Message) {
	_, ok := c.Messages[message.Timestamp]
	c.Messages[message.Timestamp] = message
	if !ok {
		// new messages
		c.MessageOrder = append(c.MessageOrder, message.Timestamp)
		c.HasNewMessage = true
	}
}

func NewConversation(contact *Contact) *Conversation {
	return &Conversation{
		Contact:       contact,
		Messages:      make(map[int64]*Message),
		MessageOrder:  make([]int64, 0),
		HasNewMessage: false,
	}
}

type SignalAPI interface {
	Send(string, string) error
	Receive() error
	OnReceived(signal.ReceivedCallback)
	OnReceipt(signal.ReceiptCallback)
}

type Siggo struct {
	config        *Config
	contacts      map[string]*Contact
	conversations map[*Contact]*Conversation
	signal        SignalAPI

	NewInfo func(*Conversation)
}

// Send sends a message to a contact.
func (s *Siggo) Send(msg string, contact *Contact) error {
	// update for whoever wants to know
	// ui might want to know immediately
	conv, ok := s.conversations[contact]
	if !ok {
		conv = s.newConversation(contact)
	}
	message := &Message{
		Content:     msg,
		From:        s.config.UserName,
		Timestamp:   0,
		IsDelivered: false,
		IsRead:      false,
	}
	s.onSend(message, conv)

	// actually send the message
	return s.signal.Send(contact.Number, msg)
}

func (s *Siggo) newConversation(contact *Contact) *Conversation {
	conv := NewConversation(contact)
	s.conversations[contact] = conv
	return conv
}

func (s *Siggo) newContact(number string) *Contact {
	contact := &Contact{
		Number: number,
	}
	s.contacts[number] = contact
	return contact
}

// Receive
func (s *Siggo) Receive() error {
	return s.signal.Receive()
}

func (s *Siggo) onSend(message *Message, conv *Conversation) {}

func (s *Siggo) onReceived(msg *signal.Message) error {
	// add new message to conversation
	receiveMsg := msg.Envelope.DataMessage
	contactNumber := msg.Envelope.Source
	// if we have a name for this contact, use it
	// otherwise it will be the phone number
	c, ok := s.contacts[contactNumber]

	var fromStr string
	// TODO: fix this when i can load contact names from
	// somewhere
	if !ok {
		fromStr = contactNumber
		c = &Contact{
			Number: contactNumber,
		}
		log.Printf("New contact: %v", c)
		s.contacts[c.Number] = c
	} else if c.Name == "" {
		fromStr = contactNumber
	} else {
		fromStr = c.Name
	}
	message := &Message{
		Content:     receiveMsg.Message,
		From:        fromStr,
		Timestamp:   receiveMsg.Timestamp,
		IsDelivered: true,
		IsRead:      false,
	}
	conv, ok := s.conversations[c]
	if !ok {
		log.Printf("new conversation for contact: %v", c)
		conv = s.newConversation(c)
	}
	conv.AddMessage(message)
	s.NewInfo(conv)
	return nil
}

func (s *Siggo) onReceipt(msg *signal.Message) error {
	receiptMsg := msg.Envelope.ReceiptMessage
	//fmt.Printf("RECEIPT Received:\n")
	//fmt.Printf("  From: %s\n", msg.Envelope.Source)
	//fmt.Printf("  Delivered: %t\n", receiptMsg.IsDelivery)
	//fmt.Printf("  Read: %t\n", receiptMsg.IsRead)
	//fmt.Printf("  Timestamps: %v\n", receiptMsg.Timestamps)
	// if the message exists, edit it with new data
	contactNumber := msg.Envelope.Source
	// if we have a name for this contact, use it
	// otherwise it will be the phone number
	c, ok := s.contacts[contactNumber]
	if !ok {
		c = s.newContact(contactNumber)
	}
	conv, ok := s.conversations[c]
	if !ok {
		conv = s.newConversation(c)
	}
	for _, ts := range receiptMsg.Timestamps {
		message, ok := conv.Messages[ts]
		if !ok {
			// TODO: handle case where we get a read receipt for
			// a message that we don't have
			continue
		}
		message.IsDelivered = receiptMsg.IsDelivery
		message.IsRead = receiptMsg.IsRead
	}
	return nil
}

func (s *Siggo) Conversations() map[*Contact]*Conversation {
	return s.conversations
}

func (s *Siggo) Contacts() map[string]*Contact {
	return s.contacts
}

// NewSiggo creates a new model
func NewSiggo(sig SignalAPI, config *Config) *Siggo {
	contacts := GetContacts(config.UserNumber)
	conversations := GetConversations(config.UserNumber, contacts)
	s := &Siggo{
		config:        config,
		contacts:      contacts,
		conversations: conversations,
		signal:        sig,

		NewInfo: func(*Conversation) {}, // noop
	}
	//sig.OnMessage(s.?)
	//sig.OnSent(s.?)

	sig.OnReceived(s.onReceived)
	sig.OnReceipt(s.onReceipt)
	return s
}

// GetContacts reads the contact list from disk for a given user
func GetContacts(userNumber string) map[string]*Contact {
	list := make(map[string]*Contact)
	list[userNumber] = &Contact{
		Number: userNumber,
		Name:   "me",
	}
	return list
}

// GetConversations reads conversations from disk for a given user
// and contact list
func GetConversations(userNumber string, contacts map[string]*Contact) map[*Contact]*Conversation {
	conversations := make(map[*Contact]*Conversation)
	for _, contact := range contacts {
		fmt.Printf("Adding conversation for: %+v\n", contact)
		conv := NewConversation(contact)
		conversations[contact] = conv
	}
	return conversations
}
