package main

import (
	"fmt"
	forecast "github.com/mlbright/forecast/v2"
	"log"
	"os"
	"strings"

	"encoding/json"
	"github.com/nlopes/slack"
	"io/ioutil"
)

//structure pour récupérer le token
type Token struct {
	SlackToken   string `json:"slacktoken"`
	DarkSkyToken string `json:"darkskytoken"`
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
	var darkskyToken = os.Getenv("darkskyToken")
	if slackToken == "" || darkskyToken == "" {
		logger.Printf("INFO : le token pour slack ou darksky n'existe pas dans les variables d'environnements => utilisation de token.json")
		file, err := ioutil.ReadFile("./token.json")

		if err != nil {
			logger.Printf("ERROR : le fichier token.json n'existe pas")
		}

		if err := json.Unmarshal(file, &botKey); err != nil {
			logger.Printf("ERROR : Impossible de parser token.json")
		}
	} else {
		logger.Printf("INFO : Utilisation des tokens %s et %s trouvé dans les variables d'environnement", slackToken, darkskyToken)
		botKey.SlackToken = slackToken
		botKey.DarkSkyToken = darkskyToken
	}

}

//Répond à une demande d'aide du bot
func replyHelp(channelID string) {
	commands := map[string]string{
		"aide":     "Affiche la liste des commandes possibles",
		"identité": "Donne des informations sur @nabot",
		"météo":    "Affiche la météo à Igny",
	}
	fields := make([]slack.AttachmentField, 0)
	for k, v := range commands {
		fields = append(fields, slack.AttachmentField{
			Title: "@nabot " + k,
			Value: v,
		})
	}
	sendMsg("", "", "", fields, "", channelID)
}

//Répond à la météo
func replyWeather(channelID string) {

	key := botKey.DarkSkyToken

	// Igny
	lat := "48.7352"
	long := "2.2176"

	icons := map[string]string{
		"clear-day":           ":sunny:",
		"clear-night":         ":crescent_moon:",
		"rain":                ":rain_cloud:",
		"snow":                ":snowflake:",
		"sleet":               ":snow_cloud:",
		"wind":                ":wind_blowing_face:",
		"fog":                 ":fog:",
		"cloudy":              ":cloud:",
		"partly-cloudy-day":   ":barely_sunny:",
		"partly-cloudy-night": ":barely_sunny:",
		"hail":                ":snow_cloud:",
		"thunderstorm":        ":thunder_cloud_and_rain:",
		"tornado":             ":tornado:",
	}

	f, err := forecast.Get(key, lat, long, "now", forecast.CA, forecast.French)
	if err != nil {
		logger.Printf("Error:%s", err)
	}

	//récupération du courant, d'aujourd'hui et des 2 prochains jours
	title := ""
	pretext := ""
	text := fmt.Sprintf("*Cette semaine* à Igny:%s\n\n", f.Daily.Summary)
	text += fmt.Sprintf("*Actuellement* %s: %s et il fait *%.1f°C*, avec un risque de pluie *%.0f%%* de et un vent à *%.2fm/s*\n\n", icons[f.Currently.Icon], f.Currently.Summary, f.Currently.Temperature, f.Currently.PrecipProbability*100, f.Currently.WindSpeed)
	text += fmt.Sprintf("*Aujourd'hui* %s: %s et il fera entre *%.1f°C* et *%.1f°C*, avec un risque de pluie de *%.0f%%* et un vent à *%.2fm/s*\n\n", icons[f.Daily.Data[0].Icon], f.Daily.Data[0].Summary, f.Daily.Data[0].TemperatureMin, f.Daily.Data[0].TemperatureMax, f.Daily.Data[0].PrecipProbability*100, f.Daily.Data[0].WindSpeed)
	text += fmt.Sprintf("*Demain* %s: %s et il fera entre *%.1f°C* et *%.1f°C*, avec un risque de pluie de *%.0f%%* et un vent à *%.2fm/s*\n\n", icons[f.Daily.Data[1].Icon], f.Daily.Data[1].Summary, f.Daily.Data[1].TemperatureMin, f.Daily.Data[1].TemperatureMax, f.Daily.Data[1].PrecipProbability*100, f.Daily.Data[1].WindSpeed)
	text += fmt.Sprintf("*Et après-demain* %s: %s et il fera entre *%.1f°C* et *%.1f°C*, avec un risque de pluie de *%.0f%%* et un vent à *%.2fm/s*", icons[f.Daily.Data[2].Icon], f.Daily.Data[2].Summary, f.Daily.Data[2].TemperatureMin, f.Daily.Data[2].TemperatureMax, f.Daily.Data[2].PrecipProbability*100, f.Daily.Data[2].WindSpeed)
	sendMsg(title, pretext, text, nil, "", channelID)
}

//Répond à l'identité du bot
func replyIdentity(channelID string) {
	title := "Bonjour, je m'appelle @nabot et je suis un bot pour contrôler la domotique de mes maîtres"
	pretext := "Vous trouverez plus d'information sur https://github.com/jraigneau/nabot"
	sendMsg(title, pretext, "", nil, "", channelID)
}

//Analyse des messages
//TODO:
// 1. faire un substring du nom du bot
// 2. Faire une analyse "fuzzy"
// 3. prévoir de pouvoir envoyer ce qu'il y a après le mot clef : exemple pour météo
func msgAnalysis(msg string, channelID string) {
	switch msg {
	case "<@" + botID + "> aide":
		replyHelp(channelID)
	case "<@" + botID + "> identité":
		replyIdentity(channelID)
	case "<@" + botID + "> météo":
		replyWeather(channelID)
	default:
		sendMsg("", "", "Désolé je n'ai pas reconnu la commande\nUtiliser `@nabot aide` pour avoir plus d'information", nil, "#FF0000", channelID)
	}

}

//Poste un message sur le channel channelID
//TODO: revoir gestion des champs (sur mobile apparition d'un champ "vide")
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

	api = slack.New(botKey.SlackToken)

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
