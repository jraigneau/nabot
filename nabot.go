package main

import (
	"fmt"
	"github.com/jasonlvhit/gocron"
	forecast "github.com/mlbright/forecast/v2"
	"log"
	"math"
	"os"
	"strings"

	"encoding/json"
	influx "github.com/influxdata/influxdb/client/v2"
	"github.com/nlopes/slack"
	"io/ioutil"
)

//structure pour récupérer le token
//Attention aux majuscules pour le marshmaling du JSON....
type Token struct {
	SlackToken   string `json:"slacktoken"`
	DarkSkyToken string `json:"darkskytoken"`
	UsernameDB   string `json:"usernameDB"`
	PasswordDB   string `json:"passwordDB"`
	UriDB        string `json:"uriDB"`
}

//variables
var (
	api      *slack.Client
	botKey   Token
	botID    = "N/A"
	logger   *log.Logger
	clientDB influx.Client
)

// queryDB convenience function to query the database
func queryDB(cmd string, MyDB string) (res []influx.Result, err error) {

	q := influx.Query{
		Command:  cmd,
		Database: MyDB,
	}
	response, err := clientDB.Query(q)
	if err != nil {
		logger.Printf("Error: ", err)
	}
	if response.Error() != nil {
		logger.Printf("Error: ", response.Error())
	}
	res = response.Results
	return res, nil
}

//Création de la crontab
//TODO: enlever 1H pour être en UTC
func ordonnanceur() {

	s := gocron.NewScheduler()
	s.Every(1).Day().At("07:00").Do(replyWeather, "G4056ALKB")
	s.Every(1).Day().At("20:00").Do(replyWeather, "G4056ALKB")
	<-s.Start()
}

//function d'initialisation (notamment pour récupérer le token) - j'aurai pu utiliser 'init()'
func initialisation() {

	//Initialisation Logs
	logger = log.New(os.Stdout, "nabot: ", log.Ldate|log.Ltime|log.Lshortfile)

	//Initialisation variable
	var slackToken = os.Getenv("slackToken")
	var darkskyToken = os.Getenv("darkskyToken")
	var uriDB = os.Getenv("uriDB")
	var usernameDB = os.Getenv("usernameDB")
	var passwordDB = os.Getenv("passwordDB")
	if slackToken == "" || darkskyToken == "" || uriDB == "" || usernameDB == "" || passwordDB == "" {
		logger.Printf("INFO : les tokens  n'existent pas dans les variables d'environnements => utilisation de token.json")
		file, err := ioutil.ReadFile("./token.json")

		if err != nil {
			logger.Printf("ERROR : le fichier token.json n'existe pas")
			os.Exit(-1)
		}

		if err := json.Unmarshal(file, &botKey); err != nil {
			logger.Printf("ERROR : Impossible de parser token.json")
			os.Exit(-1)
		}

	} else {
		logger.Printf("INFO : Utilisation des tokens trouvé dans les variables d'environnement")
		botKey.SlackToken = slackToken
		botKey.DarkSkyToken = darkskyToken
		botKey.UriDB = uriDB
		botKey.UsernameDB = usernameDB
		botKey.PasswordDB = passwordDB
	}

	logger.Printf("INFO : Tokens utilisés", botKey.UriDB, botKey.DarkSkyToken, botKey.PasswordDB, botKey.SlackToken, botKey.UsernameDB)

	//Initialisation base
	var err error
	clientDB, err = influx.NewHTTPClient(influx.HTTPConfig{
		Addr:     botKey.UriDB,
		Username: botKey.UsernameDB,
		Password: botKey.PasswordDB,
	})
	if err != nil {
		logger.Printf("Error: ", err)
		os.Exit(-1)
	} else {
		logger.Printf("INFO : Connexion à influxDB réalisée avec succès:", clientDB)

	}

}

//Renvoie la consommation électrique
func replyConsoElectrique(channelID string) {

	q := fmt.Sprintf("SELECT * FROM energy ORDER BY time DESC LIMIT 1")
	res, err := queryDB(q, "electricity")
	if err != nil {
		logger.Printf("Error: ", err)
	}

	day_energy := res[0].Series[0].Values[0][1].(json.Number).String()
	instant_energy := res[0].Series[0].Values[0][2].(json.Number).String()

	title := ""
	pretext := ""
	text := fmt.Sprintf("Actuellement la consommation instantanée est de *%sW* et le cumul est de *%skW*.", instant_energy, day_energy)
	sendMsg(title, pretext, text, nil, "", channelID)

}

//Renvoie les métriques autour du trafic internet @home
func replyInternet(channelID string) {

	q := fmt.Sprintf("SELECT mean(\"rx\")/1000 FROM traffic where \"interface\" = 'pppoe-wan6' and time > now() - 5m")
	res, err := queryDB(q, "traffic")
	if err != nil {
		log.Fatal("Error: ", err)
	}
	mean_rx, err := res[0].Series[0].Values[0][1].(json.Number).Float64()
	if err != nil {
		log.Fatal("Error: ", err)
	}
	mean_rx = math.Floor(mean_rx)

	q2 := fmt.Sprintf("SELECT mean(\"tx\")/1000 FROM traffic where \"interface\" = 'pppoe-wan6' and time > now() - 5m")
	res2, err := queryDB(q2, "traffic")
	if err != nil {
		log.Fatal("Error: ", err)
	}
	mean_tx, err := res2[0].Series[0].Values[0][1].(json.Number).Float64()
	if err != nil {
		log.Fatal("Error: ", err)
	}
	mean_tx = math.Floor(mean_tx)

	q3 := fmt.Sprintf("SELECT mean(\"value\") FROM ping where \"site\" = 'google' and time > now() - 5m")
	res3, err := queryDB(q3, "uptime")
	if err != nil {
		log.Fatal("Error: ", err)
	}
	uptime, err := res3[0].Series[0].Values[0][1].(json.Number).Float64()
	if err != nil {
		log.Fatal("Error: ", err)
	}
	uptime = math.Floor(uptime)

	title := ""
	pretext := ""
	text := fmt.Sprintf("Sur les 5 dernières minutes, Le trafic entrant moyen est de *%vKb/s* et de *%vKb/s* en sortie. La moyenne du ping vers google est *%vms*.", mean_rx, mean_tx, uptime)
	sendMsg(title, pretext, text, nil, "", channelID)
}

//Répond à une demande d'aide du bot
func replyHelp(channelID string) {
	commands := map[string]string{
		"aide":     ":question: Liste des commandes possibles",
		"météo":    ":umbrella: Météo à Igny",
		"conso":    ":bulb: Consommation électrique",
		"internet": ":computer: Traffic internet",
		"traffic":  ":car: Traffic routier",
		"temp":     ":thermometer: Température dans la maison",
	}
	fields := make([]slack.AttachmentField, 0)
	for k, v := range commands {
		fields = append(fields, slack.AttachmentField{
			Title: "@nabot " + k,
			Value: v,
		})
	}
	title := "Bonjour, je m'appelle @nabot et je suis un bot pour contrôler la domotique de mes maîtres."
	pretext := "Vous trouverez plus d'information sur https://github.com/jraigneau/nabot"
	sendMsg(title, pretext, "", fields, "", channelID)
}

//Renvoie le traffic routier
func replyTraffic(channelID string) {

	q := fmt.Sprintf("SELECT trafficDelayInSeconds,travelTimeInSeconds FROM traffic where \"name\"='laurence' ORDER BY time DESC LIMIT 1")
	res, err := queryDB(q, "trafficy")
	if err != nil {
		log.Fatal("Error: ", err)
	}

	//Attention le nom n'est plus cohérent...
	trafficDelayInSecondsJR := res[0].Series[0].Values[0][1].(json.Number).String()
	travelTimeInSecondsJR := res[0].Series[0].Values[0][2].(json.Number).String()

	q1 := fmt.Sprintf("SELECT trafficDelayInSeconds,travelTimeInSeconds FROM traffic where \"name\"='laurence-soir' ORDER BY time DESC LIMIT 1")
	res1, err := queryDB(q1, "trafficy")
	if err != nil {
		log.Fatal("Error: ", err)
	}

	trafficDelayInSecondsLR := res1[0].Series[0].Values[0][1].(json.Number).String()
	travelTimeInSecondsLR := res1[0].Series[0].Values[0][2].(json.Number).String()

	title := ""
	pretext := ""
	text := fmt.Sprintf("Actuellement il faut *%vmin* pour aller à Aviva (*%vmin* de bouchon) et *%vmin* pour en revenir (*%vmin* de bouchon).", travelTimeInSecondsJR, trafficDelayInSecondsJR, travelTimeInSecondsLR, trafficDelayInSecondsLR)
	sendMsg(title, pretext, text, nil, "", channelID)
}

//Récupère les dernières valeurs de température
func replyTemp(channelID string) {

	q := fmt.Sprintf("SELECT * FROM temperature ORDER BY time DESC LIMIT 15")
	res, err := queryDB(q, "tempDB")
	if err != nil {
		log.Fatal("Error: ", err)
	}

	var temperatures = make(map[string]string)
	for row := range res[0].Series[0].Values {
		name := res[0].Series[0].Values[row][1].(string)
		val := res[0].Series[0].Values[row][2].(json.Number).String()
		_, ok := temperatures[name] //vérifie la présence de la pièce dans la map
		if !ok {
			temperatures[name] = val
		}
	}

	q1 := fmt.Sprintf("select value from hygrometrie order by time desc limit 1")
	res1, err := queryDB(q1, "hygroDB")
	if err != nil {
		log.Fatal("Error: ", err)
	}

	hygroSDB := res1[0].Series[0].Values[0][1].(json.Number).String()

	var result = "Les températures des pièces sont:"
	for room := range temperatures {
		result = fmt.Sprintf("%s\n- %s: *%s°C*", result, room, temperatures[room])
	}

	fields := make([]slack.AttachmentField, 0)
	for room := range temperatures {
		var temp = temperatures[room] + "°C"
		if room == "Douche" {
			temp = temp + " (humidité: " + hygroSDB + "%)"
		}
		fields = append(fields, slack.AttachmentField{
			Title: room,
			Value: temp,
			Short: true,
		})
	}

	result = fmt.Sprintf("%s\net le degré d'humidité dans le SdB est de *%v%%*.", result, hygroSDB)

	title := ""
	pretext := ""
	text := ""
	sendMsg(title, pretext, text, fields, "", channelID)
}

//Répond à la météo
//TODO: prévoir gestion des erreurs sur les icones pour éviter plantage
//TODO: prévoir la gestion d'autres villes
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
	text := fmt.Sprintf("*Cette semaine* à Igny: %s\n\n", f.Daily.Summary)
	text += fmt.Sprintf("*Actuellement* %s: %s et il fait *%.1f°C*, avec un risque de pluie de *%.0f%%* et un vent à *%.0fm/s*\n\n", icons[f.Currently.Icon], f.Currently.Summary, f.Currently.Temperature, f.Currently.PrecipProbability*100, f.Currently.WindSpeed)
	text += fmt.Sprintf("*Aujourd'hui* %s: %s et il fera entre *%.1f°C* et *%.1f°C*, avec un risque de pluie de *%.0f%%* et un vent à *%.0fm/s*\n\n", icons[f.Daily.Data[0].Icon], f.Daily.Data[0].Summary, f.Daily.Data[0].TemperatureMin, f.Daily.Data[0].TemperatureMax, f.Daily.Data[0].PrecipProbability*100, f.Daily.Data[0].WindSpeed)
	text += fmt.Sprintf("*Demain* %s: %s et il fera entre *%.1f°C* et *%.1f°C*, avec un risque de pluie de *%.0f%%* et un vent à *%.0fm/s*\n\n", icons[f.Daily.Data[1].Icon], f.Daily.Data[1].Summary, f.Daily.Data[1].TemperatureMin, f.Daily.Data[1].TemperatureMax, f.Daily.Data[1].PrecipProbability*100, f.Daily.Data[1].WindSpeed)
	text += fmt.Sprintf("*Et après-demain* %s: %s et il fera entre *%.1f°C* et *%.1f°C*, avec un risque de pluie de *%.0f%%* et un vent à *%.0fm/s*", icons[f.Daily.Data[2].Icon], f.Daily.Data[2].Summary, f.Daily.Data[2].TemperatureMin, f.Daily.Data[2].TemperatureMax, f.Daily.Data[2].PrecipProbability*100, f.Daily.Data[2].WindSpeed)
	sendMsg(title, pretext, text, nil, "", channelID)
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
	case "<@" + botID + "> météo":
		replyWeather(channelID)
	case "<@" + botID + "> conso":
		replyConsoElectrique(channelID)
	case "<@" + botID + "> internet":
		replyInternet(channelID)
	case "<@" + botID + "> traffic":
		replyTraffic(channelID)
	case "<@" + botID + "> temp":
		replyTemp(channelID)
	default:
		sendMsg("", "", "Désolé je n'ai pas reconnu la commande\nUtilisez `@nabot aide` pour avoir plus d'information", nil, "#FF0000", channelID)
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

	api = slack.New(botKey.SlackToken)

	slack.SetLogger(logger)
	api.SetDebug(false)

	//go ordonnanceur()

	rtm := api.NewRTM()
	go rtm.ManageConnection()

	var firstConnection = 0

	for msg := range rtm.IncomingEvents {
		//fmt.Print("Event Received: ")
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			// Ignore hello

		case *slack.ConnectedEvent:
			logger.Print("Infos:", ev.Info)
			botID = ev.Info.User.ID
			if firstConnection == 0 {
				sendMsg("", "", "Heigh-ho, heigh-ho je vais au boulot", nil, "", "G4056ALKB")
				firstConnection = 1 //pour éviter de renvoyer le message avec une connexion instable...
			}

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
