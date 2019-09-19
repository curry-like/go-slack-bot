package slack_bot

import (
	"bytes"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"github.com/ikawaha/kagome/tokenizer"
	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"log"
	"net/http"
	"os"
	"strings"
)

type Dictionary struct {
	Ubiquitous string
	Synonym    string
}

type Response struct {
	Ubiquitous string
	Answer     string
}

type History struct {
	EventId string
	Text    string
}

var verificationToken = os.Getenv("VERIFICATION_TOKEN")
var api = slack.New(os.Getenv("SLACK_TOKEN"))

var ctx = context.Background()
var projectId = os.Getenv("PROJECT_ID")
var client, _ = datastore.NewClient(ctx, projectId)

func SlackBot(w http.ResponseWriter, r *http.Request) {

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)
	body := buf.String()
	log.Print("requestbody: " + body)
	eventsAPIEvent, e := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: verificationToken}))
	if e != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text")
		_, _ = w.Write([]byte(r.Challenge))

	case slackevents.CallbackEvent:
		event := eventsAPIEvent.Data.(*slackevents.EventsAPICallbackEvent)
		id := event.EventID
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			if isDuplicate(id, ev.Text) {
				return
			}
			message := createMessage(ev.Text, ev.User)
			_, _, e := api.PostMessage(ev.Channel, slack.MsgOptionText(message, false))
			if e != nil {
				log.Print(e)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

		case *slackevents.MessageEvent:
			if ev.ChannelType == "im" && ev.BotID != "BL29A809Y" && !isDuplicate(id, ev.Text) {
				log.Print(ev.Text)
				message := createMessage(ev.Text, ev.User)
				_, _, e := api.PostMessage(ev.Channel, slack.MsgOptionText(message, false))
				if e != nil {
					log.Print(e)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func isDuplicate(eventId string, text string) bool {
	var history History
	_, e := history.Get(ctx, eventId)
	if e != nil {
		history = History{
			EventId: eventId,
			Text:    text,
		}
		_, _ = history.Save(ctx)
		log.Print(e)
		return false
	} else {
		return true
	}
}

func createMessage(text string, user string) string {
	mention := "<@" + user + ">"
	var message string
	var responses []Response
	tokens := tokenize(text)
	for _, token := range tokens {
		dictionary := convertToDictionary(token)
		log.Print(dictionary.Ubiquitous)
		r, err := getResponseInfo(dictionary.Ubiquitous)
		if err != nil {
			continue
		}
		responses = append(responses, r)
	}
	if len(responses) == 0 {
		message = "質問の意味が分かりませんでした。申し訳ありませんがプレミアムチームに直接お問い合わせをお願いいたします。"
	} else {
		for _, response := range responses {
			message += response.Ubiquitous + ": " + "\r\n" + response.Answer + "\r\n\r\n"
		}
	}
	return mention + "\r\n" + message
}

func tokenize(text string) []string {
	t := setupDictionary()
	tokens := t.Tokenize(text)
	var wordList []string
	for _, token := range tokens {
		if token.Class == tokenizer.DUMMY {
			continue
		}
		if strings.Contains(token.Pos(), "名詞") {
			wordList = append(wordList, token.Surface)
		}
		token.Features()
	}
	return wordList
}

func setupDictionary() tokenizer.Tokenizer {
	t := tokenizer.New()
	udic, err := NewUserDic()
	if err != nil {
		log.Print("An error occurred in reading dictionary.txt.", err)
	}
	t.SetUserDic(udic)
	return t
}

// read dictionary from gs
func NewUserDic() (tokenizer.UserDic, error) {
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
	 	return tokenizer.UserDic{}, err
	}
	defer storageClient.Close()

	r, err := storageClient.Bucket("slack-bot").Object("dictionary.txt").NewReader(ctx)

	defer r.Close()

	records, err := tokenizer.NewUserDicRecords(r)
	if err != nil {
		return tokenizer.UserDic{}, err
	}
	return records.NewUserDic()
}

func convertToDictionary(synonym string) Dictionary {
	log.Print("synonym", synonym)
	var dictionary Dictionary
	_, e := dictionary.Get(ctx, synonym)
	if e != nil {
		return Dictionary{
			Ubiquitous: synonym,
			Synonym:    synonym,
		}
	}
	return dictionary
}

func getResponseInfo(ubiquitous string) (Response, error) {
	log.Print("response", ubiquitous)
	var response Response
	_, e := response.Get(ctx, ubiquitous)
	if e != nil {
		return Response{}, e
	}
	return response, nil
}

func (history *History) Get(ctx context.Context, id string) (*datastore.Key, error) {
	q := datastore.NewQuery("History").Filter("EventId =", id)
	it := client.Run(ctx, q)
	return it.Next(history)
}

func (history *History) Save(ctx context.Context) (*datastore.Key, error) {
	return client.Put(ctx, datastore.IncompleteKey("History", nil), history)
}

func (dictionary *Dictionary) Get(ctx context.Context, synonym string) (*datastore.Key, error) {
	q := datastore.NewQuery("Dictionary").Filter("Synonym =", synonym)
	it := client.Run(ctx, q)
	return it.Next(dictionary)
}

func (response *Response) Get(ctx context.Context, ubiquitous string) (*datastore.Key, error) {
	q := datastore.NewQuery("Response").Filter("Ubiquitous =", ubiquitous)
	it := client.Run(ctx, q)
	return it.Next(response)
}
