package main

type Update struct {
	UpdateID      int      `json:"update_id"`
	Message       *Message `json:"message"`
	EditedMessage *Message `json:"edited_message"`
	ChannelPost   *Message `json:"channel_post"`
}

type Chat struct {
	Id        int    `json:id`
	Type      string `json:type`
	Title     string `json:title`
	Username  string `json:username`
	FirstName string `json:first_name`
	LastName  string `json:last_name`
	AllAdmin  bool   `json:all_members_are_administrator`
}
type User struct {
	Id         int    `json:int`
	First_name string `json:first_name`
	LastName   string `json:last_name`
	Username   string `json:username`
	Language   string `json:language_code`
}

type MessageEntities struct {
	Type   string `json:type`
	Offset int    `json:offset`
	Length int    `json:length`
	Url    string `json:url`
	User   User   `json:user`
}

type Message struct {
	MessageId int               `json:message_id`
	From      User              `json:from`
	Date      int               `json:date`
	Chat      Chat              `json:chat`
	Text      string            `json:text`
	Enitities []MessageEntities `json:entities`
}
