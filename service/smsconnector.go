package service

import (
	"errors"
	"go-kafka-alert/db"
	"time"
	"strconv"
	"regexp"
	"github.com/sfreiberg/gotwilio"
	"go-kafka-alert/util"
)

type EventForSMS struct {
	TriggeredEvent db.Event
}

func (event EventForSMS) ParseTemplate() ([]db.Message, error) {
	var messages []db.Message
	channelSupported := CheckChannel(event.TriggeredEvent, "SMS")
	if !channelSupported {
		util.Trace.Println("Dropping event ['" + event.TriggeredEvent.EventId + "']. SMS channel not supported.")
		return messages, errors.New("SMS channel not supported")
	}
	numOfRecipient := len(event.TriggeredEvent.Recipient)
	if numOfRecipient <= 0 {
		util.Trace.Println("Dropping event ['" + event.TriggeredEvent.EventId + "']. No recipient found.")
		return messages, errors.New("No recipients found")
	}
	var messageContent, _ = ParseTemplateForMessage(event.TriggeredEvent, "SMS")

	//generate individual messages for each recipient
	for _, rep := range event.TriggeredEvent.Recipient {
		if validatePhone(rep) {
			dateCreated := time.Now()
			message := db.Message{}
			message.AlertId = strconv.Itoa(dateCreated.Nanosecond()) + rep + event.TriggeredEvent.EventId
			message.Subject = event.TriggeredEvent.Subject
			message.Reference = event.TriggeredEvent.EventId+"SMS"
			message.Content = messageContent + " " + rep //temp
			message.Recipient = rep
			message.DateCreated = dateCreated
			message.MessageId = strconv.Itoa(dateCreated.Nanosecond()) + rep + event.TriggeredEvent.EventId
			messages = append(messages, message)
		} else {
			util.Error.Println("Phone number not valid ['" + rep + "']")
		}
	}
	return messages, nil
}

func (event EventForSMS) SendMessage(message db.Message) db.MessageResponse {
	var response = db.MessageResponse{}
	if (db.Message{}) == message {
		util.Error.Println("Sending  Failed. Message body empty")
		return db.MessageResponse{Status:util.FAILED, Response:"MESSAGE EMPTY", TimeOfResponse: time.Now()}
	}
	if message.Content == "" {
		util.Error.Println("Sending  Failed. Message body empty")
		return db.MessageResponse{Status:util.FAILED, Response:"MESSAGE HAS NO CONTENT", TimeOfResponse: time.Now()}
	}
	if util.AppConfiguration.SmsConfig.UserName == "" || util.AppConfiguration.SmsConfig.Password == "" ||
		util.AppConfiguration.SmsConfig.SenderName == "" {
		util.Error.Println("Sending  Failed. SMS Config not available")
		return db.MessageResponse{Status:util.FAILED, Response:"SMS Config not available", TimeOfResponse: time.Now()}
	}
	twilio := gotwilio.NewTwilioClient(util.AppConfiguration.SmsConfig.UserName, util.AppConfiguration.SmsConfig.Password)
	twilioSmsResponse, smsEx, _ := twilio.SendSMS(util.AppConfiguration.SmsConfig.SenderName, message.Recipient, message.Content, "", "")
	if smsEx != nil {
		response.Response = smsEx.Message
		response.APIStatus = strconv.Itoa(smsEx.Status)
		response.Status = util.SUCCESS
		response.TimeOfResponse = time.Now()
		util.Info.Println("SMS sent to  ['" + message.Recipient + "']")
		return response
	}
	timeSent, err := twilioSmsResponse.DateSentAsTime()
	if err != nil {
		timeSent = time.Now()
		util.Error.Println("Sending  Failed. " + err.Error())
	}
	response.Response = twilioSmsResponse.Body
	response.APIStatus = strconv.Itoa(smsEx.Status)
	response.Status = util.FAILED
	response.TimeOfResponse = timeSent
	return response
}

func validatePhone(phone string) bool {
	re := regexp.MustCompile("[0-9]+") //temporal regex
	return re.MatchString(phone)
}

