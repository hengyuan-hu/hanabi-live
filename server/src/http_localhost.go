// The Hanabi server also listens on a separate port that only accepts connections from the local
// system; this allows administrative tasks to be performed without having to go through a browser

package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"fmt"
  "math/rand"
	"github.com/gin-gonic/gin"
)

func httpLocalhostInit() {
	// Read some configuration values from environment variables
	// (they were loaded from the ".env" file in "main.go")
	portString := os.Getenv("LOCALHOST_PORT")
	var port int
	if len(portString) == 0 {
		port = 8081
	} else {
		if v, err := strconv.Atoi(portString); err != nil {
			logger.Fatal("Failed to convert the \"LOCALHOST_PORT\" " +
				"environment variable to a number.")
			return
		} else {
			port = v
		}
	}

	// Create a new Gin HTTP router
	gin.SetMode(gin.ReleaseMode)
	httpRouter := gin.Default() // Has the "Logger" and "Recovery" middleware attached

	// Path handlers
	httpRouter.POST("/ban", httpLocalhostUserAction)
	httpRouter.GET("/cancel", httpLocalhostCancel)
	httpRouter.GET("/clearEmptyTables", httpLocalhostClearEmptyTables)
	httpRouter.GET("/debug", httpLocalhostDebug)
	httpRouter.GET("/maintenance", httpLocalhostMaintenance)
	httpRouter.POST("/mute", httpLocalhostUserAction)
	httpRouter.GET("/print", httpLocalhostPrint)
	httpRouter.GET("/restart", httpLocalhostRestart)
	httpRouter.GET("/saveTables", httpLocalhostSaveTables)
	httpRouter.POST("/sendWarning", httpLocalhostUserAction)
	httpRouter.POST("/sendError", httpLocalhostUserAction)
	httpRouter.POST("/serialize", httpLocalhostSerialize)
	httpRouter.GET("/shutdown", httpLocalhostShutdown)
	httpRouter.GET("/timeLeft", httpLocalhostTimeLeft)
	httpRouter.GET("/uptime", httpLocalhostUptime)
	httpRouter.GET("/version", httpLocalhostVersion)
	httpRouter.GET("/unmaintenance", httpLocalhostUnmaintenance)
	httpRouter.GET("/createRoom", httpLocalhostCreateRoom)

	// We need to create a new http.Server because the default one has no timeouts
	// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	HTTPServerWithTimeout := &http.Server{
		Addr:         "127.0.0.1:" + strconv.Itoa(port), // Listen only on the localhost interface
		Handler:      httpRouter,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if err := HTTPServerWithTimeout.ListenAndServe(); err != nil {
		logger.Fatal("ListenAndServe failed (for localhost):", err)
		return
	}
	logger.Fatal("ListenAndServe ended prematurely (for localhost).")
}

// var hasPlayedBot = make(map[string]bool)
var hasPlayedWith = make(map[string]map[string]bool)
var hasPlayedSeed = make(map[string]bool)
var humanSeeds = []int{}
var botSeeds = []int{}

func generate_unique_seeds() {
	keys := make(map[int]bool)
	seed := rand.NewSource(1)
	rng := rand.New(seed)
	for len(humanSeeds) < 10000 {
		n := rng.Intn(1000000000)
		if _, ok := keys[n]; !ok {
			keys[n] = true
			humanSeeds = append(humanSeeds, n)
			botSeeds = append(botSeeds, n)
		}
	}
	fmt.Println("Generated ", len(humanSeeds), " seeds.")
}

func seperateBotHuman(botName string) (*Session, []*Session) {
	var bot *Session = nil
	var humans = []*Session{}

	for _, s := range sessions {
		if strings.HasPrefix(s.Username(), botName) {
			if bot != nil {
				fmt.Println("Warning: More than 1 bot, ignore ", s.Username())
			} else {
				bot = s
			}
		} else {
			humans = append(humans, s)
		}
	}
	return bot, humans;
}

func initMaps(sessions map[int]*Session) {
	for _, s := range sessions {
		if _, ok := hasPlayedWith[s.Username()]; !ok {
			hasPlayedWith[s.Username()] = make(map[string]bool)
		}
	}
}

func unattendReplayAndGetAvailHumans(humans []*Session) []*Session {
	availHumans := []*Session{}
	for _, s := range humans {
		// if this guy is in a game
		if t := s.GetJoinedTable(); t != nil {
			if t.Replay {
				var cmdData CommandData
				cmdData.TableID = t.ID
				commandTableUnattend(s, &cmdData)
				availHumans = append(availHumans, s)
			}
		} else {
			availHumans = append(availHumans, s)
		}
	}
	return availHumans
}

func decideHumanPlayBot(bot *Session, humans []*Session) ([]*Session, []*Session) {
	// humans here should all be available
	if bot == nil {
		return []*Session{}, humans
	}

	willPlayBot := []*Session{}
	rest := []*Session{}

	numAvailHuman := len(humans)
	var numBotGame int = numAvailHuman / 3
	if (numAvailHuman - numBotGame) % 2 == 1 {
		numBotGame += 1
	}

	if numBotGame == 0 {
		return []*Session{}, humans
	}

	for _, s := range humans {
		if numBotGame <= 0 || hasPlayedWith[s.Username()][bot.Username()] {
			rest = append(rest, s)
		} else {
			willPlayBot = append(willPlayBot, s)
			numBotGame -= 1
			if hasPlayedWith[bot.Username()][s.Username()] {
				panic("Error: bot has already played with this guy")
			}
			hasPlayedWith[s.Username()][bot.Username()] = true
			hasPlayedWith[bot.Username()][s.Username()] = true
		}
	}

	if len(willPlayBot) + len(rest) != len(humans) {
		panic("Error in decideHumanPlayBot")
	}
	return willPlayBot, rest
}

func formHumanPlayPairs(humans []*Session) ([][]*Session, []*Session) {
	pairs := [][]*Session{}
	for _, s := range humans {
		paired := false
		for i, _ := range(pairs) {
			if len(pairs[i]) == 2 {
				continue
			}

			if len(pairs[i]) != 1 {
				panic("Bug in formHumanPlayPairs")
			}

			creator := pairs[i][0]
			creatorName := creator.Username()

			if hasPlayedWith[creatorName][s.Username()] {
				if !hasPlayedWith[s.Username()][creatorName] {
					panic("Error: wrong bookkeeping in hasPayedWith")
				}
				continue
			}

			// finally, find a possible pair
			hasPlayedWith[creatorName][s.Username()] = true
			hasPlayedWith[s.Username()][creatorName] = true
			pairs[i] = append(pairs[i], s)
			paired = true
			break
		}

		if !paired {
			// create a new pair
			pairs = append(pairs, []*Session{s})
		}
	}

	// remove unpaired groups
	validPairs := [][]*Session{}
	rest := []*Session{}
	for _, pair := range(pairs) {
		if len(pair) == 2 {
			validPairs = append(validPairs, pair)
		} else {
			rest = append(rest, pair[0])
		}
	}
	fmt.Println(">>>>>LOG:", len(humans), "players form", len(validPairs),
		"pairs and remains", len(rest), "singles")
	return validPairs, rest
}

func createRoom(p1 *Session, p2 *Session, seeds []int) []int {
	seedIdx := -1
	for i, seed := range seeds {
		seedString := strconv.Itoa(seed)
		key1 := p1.Username() + "-seed" + seedString
		key2 := p2.Username() + "-seed" + seedString
		if _, ok := hasPlayedSeed[key1]; ok {
			continue
		}
		if _, ok := hasPlayedSeed[key2]; ok {
			continue
		}

		hasPlayedSeed[key1] = true
		hasPlayedSeed[key2] = true
		seedIdx = i
		break
	}

	seed := seeds[seedIdx]
	seeds = append(seeds[:seedIdx], seeds[seedIdx+1:]...)

	fmt.Println(">>>>>LOG: create table with seed:", seed,
		", for[", p1.Username(), "] and [", p2.Username(), "]")

	var cmdData CommandData
	cmdData.Name = "!seed " + strconv.Itoa(seed)
	commandTableCreate(p1, &cmdData)
	commandTableJoin(p2, &cmdData)
	commandTableStart(p1, &cmdData)

	return seeds
}

func createHumanRooms(pairs [][]*Session) {
	for _, pair := range(pairs) {
		humanSeeds = createRoom(pair[0], pair[1], humanSeeds)
	}
}

func createBotRooms(humans []*Session, bot *Session) {
	for _, human := range(humans) {
		botSeeds = createRoom(human, bot, botSeeds)
	}
}

func httpLocalhostCreateRoom(c *gin.Context) {
	fmt.Println(len(sessions), "sessions online in total")

	initMaps(sessions)
	if len(humanSeeds) == 0 {
		generate_unique_seeds()
	}

	bot, humans := seperateBotHuman("Bot-")
	humans = unattendReplayAndGetAvailHumans(humans)
	fmt.Println(">>>>>LOG: num avail human: ", len(humans))
	willPlayBot, rest := decideHumanPlayBot(bot, humans)
	fmt.Println(">>>>>LOG: # human will play bot: ", len(willPlayBot))
	humanPairs, rest := formHumanPlayPairs(rest)

	createHumanRooms(humanPairs)

	if (bot != nil) {
		// rest will be force to play with bot
		for _, s := range(rest) {
			if !hasPlayedWith[s.Username()][bot.Username()] {
				hasPlayedWith[s.Username()][bot.Username()] = true
				hasPlayedWith[bot.Username()][s.Username()] = true
			}
		}
		willPlayBot = append(willPlayBot, rest...)
		createBotRooms(willPlayBot, bot)
	}
}

func httpLocalhostUserAction(c *gin.Context) {
	// Local variables
	w := c.Writer

	// Validate the username
	username := c.PostForm("username")
	if username == "" {
		http.Error(w, "Error: You must specify a username.", http.StatusBadRequest)
		return
	}

	// Check to see if this username exists in the database
	var userID int
	if exists, v, err := models.Users.Get(username); err != nil {
		logger.Error("Failed to get user \""+username+"\":", err)
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
		return
	} else if !exists {
		c.String(http.StatusOK, "User \""+username+"\" does not exist in the database.\n")
		return
	} else {
		userID = v.ID
	}

	// Get the IP for this user
	var lastIP string
	if v, err := models.Users.GetLastIP(username); err != nil {
		logger.Error("Failed to get the last IP for \""+username+"\":", err)
		return
	} else {
		lastIP = v
	}

	path := c.FullPath()
	if strings.HasPrefix(path, "/ban") {
		httpLocalhostBan(c, username, lastIP, userID)
	} else if strings.HasPrefix(path, "/mute") {
		httpLocalhostMute(c, username, lastIP, userID)
	} else if strings.HasPrefix(path, "/sendWarning") {
		httpLocalhostSendWarning(c, userID)
	} else if strings.HasPrefix(path, "/sendError") {
		httpLocalhostSendError(c, userID)
	} else {
		http.Error(w, "Error: Invalid URL.", http.StatusNotFound)
	}
}

func logoutUser(userID int) {
	if s, ok := sessions[userID]; !ok {
		logger.Info("Attempted to manually log out user " + strconv.Itoa(userID) + ", " +
			"but they were not online.")
	} else {
		if err := s.Close(); err != nil {
			logger.Info("Failed to manually close the WebSocket session for user "+
				strconv.Itoa(userID)+":", err)
		} else {
			logger.Info("Successfully terminated the WebSocket session for user " +
				strconv.Itoa(userID) + ".")
		}
	}
}
