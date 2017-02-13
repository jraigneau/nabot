package main

import (
	"log"
	"os"
	"strings"

	"encoding/json"
	"github.com/nlopes/slack"
	"io/ioutil"
)

//structure pour récupérer le token
type Token struct {
	Token string `json:"token"`
}

//variables
var (
	api    *slack.Client
	botKey Token
	botID  = "N/A"
	logger *log.Logger
)

//function d'initialisation (notamment pour récupérer le token) - j'aurai pu utiliser 'init()'
func initialisation() {

	//Initialisation Logs
	logger = log.New(os.Stdout, "nabot: ", log.Ldate|log.Ltime|log.Lshortfile)

	//Initialisation variable
	var slackToken = os.Getenv("slackToken")
	if slackToken == "" {
		logger.Printf("INFO : le token pour slack n'existe pas dans les variables d'environnements => utilisation de token.json")
		file, err := ioutil.ReadFile("./token.json")

		if err != nil {
			logger.Printf("ERROR : le fichier token.json n'existe pas")
		}

		if err := json.Unmarshal(file, &botKey); err != nil {
			logger.Printf("ERROR : Impossible de parser token.json")
		}
	} else {
		logger.Printf("INFO : Utilisation du token %s trouvé dans les variables d'environnement", slackToken)
		botKey.Token = slackToken
	}

}

//Répond à une demande d'aide du bot
func replyHelp(channelID string) {
	commands := map[string]string{
		"aide":     "Affiche la liste des commandes possibles",
		"identité": "Donne des informations sur @nabot"}
	fields := make([]slack.AttachmentField, 0)
	for k, v := range commands {
		fields = append(fields, slack.AttachmentField{
			Title: "@nabot " + k,
			Value: v,
		})
	}
	sendMsg("Aide", "", "", fields, "", channelID)
}

//Répond à l'identité du bot
func replyIdentity(channelID string) {
	var title = "Bonjour, je m'appelle @nabot et je suis un bot pour contrôler la domotique de mes maîtres"
	var pretext = "Vous trouverez plus d'information sur https://github.com/jraigneau/nabot"
	sendMsg(title, pretext, "", nil, "", channelID)
}

func msgAnalysis(msg string, channelID string) {
	switch msg {
	case "<@" + botID + "> aide":
		replyHelp(channelID)
	case "<@" + botID + "> identité":
		replyIdentity(channelID)
	default:
		sendMsg("Désolé je n'ai pas reconnu la commande", "", "Utiliser `@nabot aide` pour avoir plus d'information", nil, "#FF0000", channelID)
	}

}

//Poste un message sur le channel channelID
func sendMsg(title string, pretext string, text string, fields []slack.AttachmentField, colorMsg string, channelID string) {
	var color = "#B733FF" // couleur par défaut
	if colorMsg != "" {
		color = colorMsg
	}

	params := slack.PostMessageParameters{}
	params.AsUser = true
	attachment := slack.Attachment{
		Pretext:    pretext,
		Color:      color,
		Text:       text,
		Fields:     fields,
		MarkdownIn: []string{"text", "pretext", "fields"},
	}
	params.Attachments = []slack.Attachment{attachment}
	_, _, err := api.PostMessage(channelID, title, params)
	if err != nil {
		logger.Printf("%s\n", err)
		return
	}
	logger.Printf("Message envoyé avec succès sur le channel %s", channelID)

}

func main() {

	initialisation()

	api = slack.New(botKey.Token)

	slack.SetLogger(logger)
	api.SetDebug(false)

	rtm := api.NewRTM()
	go rtm.ManageConnection()

	for msg := range rtm.IncomingEvents {
		//fmt.Print("Event Received: ")
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			// Ignore hello

		case *slack.ConnectedEvent:
			logger.Print("Infos:", ev.Info)
			botID = ev.Info.User.ID
			sendMsg("Heigh-ho, heigh-ho je vais au boulot", "", "", nil, "", "G4056ALKB")

		case *slack.MessageEvent:
			if ev.Type == "message" && strings.HasPrefix(ev.Text, "<@"+botID+">") {
				msgAnalysis(ev.Text, ev.Channel)
			}
			logger.Printf("Message: %v\n", ev)

		case *slack.PresenceChangeEvent:
			//logger.Printf("Presence Change: %v\n", ev)

		case *slack.LatencyReport:
			//logger.Printf("Current latency: %v\n", ev.Value)

		case *slack.RTMError:
			logger.Printf("Error: %s\n", ev.Error())

		case *slack.InvalidAuthEvent:
			logger.Printf("Invalid credentials")
			return

		default:

			// Ignore other events..
			// logger.Printf("Unexpected: %v\n", msg.Data)
		}
	}
}
