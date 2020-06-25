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

var hasPlayedBot = make(map[string]bool)
var hasPlayedWith = make(map[string]map[string]bool)

func httpLocalhostCreateRoom(c *gin.Context) {
	fmt.Println("this function is called")
	// TODO: need to check whether a session is playing a game, see commandTableCreate
	// TODO: think about matching strategy
	// TODO: make sure that when human paired with bot, human create the room
	fmt.Println(len(sessions), "sessions")

	var bot *Session = nil

	var numAvailHuman int = 0
	// get all available human players
	for _, s := range sessions {
		if strings.HasPrefix(s.Username(), "Bot-") {
			if bot != nil {
				fmt.Println("Warning: More than 1 bot, ignore ", s.Username())
			} else {
				bot = s
			}
			continue
		}
		if _, ok := hasPlayedBot[s.Username()]; !ok {
			hasPlayedBot[s.Username()] = false
			hasPlayedWith[s.Username()] = make(map[string]bool)
		}
		if t := s.GetJoinedTable(); t != nil {
			continue
		}
		numAvailHuman += 1
	}

	if bot == nil {
		panic("Error: Cannot find bot")
	}

	var numBotGame int = numAvailHuman / 3
	if (numAvailHuman - numBotGame) % 2 == 1 {
		numBotGame += 1
	}

	humanTables : make(map[string]CommandData)
	// tableCreators : make([]*Session, 0)

	errorSessions : make([]*Session, 0)
	for _, s := range sessions {
		if strings.HasPrefix(s.Username(), "Bot-") {
			continue
		}
		if t := s.GetJoinedTable(); t != nil {
			continue
		}

		playedBot, okBot := hasPlayedBot[s.Username()]
		if !okBot {
			fmt.Println("Warning: cannot find ", s.Username(), " in playedBot")
			errorSessions = append(errorSessions, s)
		}

		var playBot := false
		if playedBot {
			fmt.Println("Log: ", s.Username(), " has played with bot")
		} else {
			fmt.Println("Log: ", s.Username(), " has not played with bot")
			if numBotGame > 0 {
				fmt.Println("Log: ", s.Username(), " wil play bot this time")
				playBot = true
				numBotGame -= 1
			} else {
				fmt.Println("Log: ", s.Username(), " wil not play bot this time")
				playBot = true
			}
		}

		// this guy will play with bot
		if playBot {
			var cmdData CommandData
			cmdData.Variant = "No Variant"
			commandTableCreate(s, &cmdData)
			commandTableJoin(bot, &cmdData)
			continue
		}

		playedWith, okPlay := hasPlayedWith[s.Username()]
		if !okPlay {
			fmt.Println("Warning: cannot find ", s.Username(), " in playedWith")
			errorSessions = append(errorSessions, s)
			continue
		}

		// this guy may join an existing table
		if len(humanTables) > 0 {
			var tableJoined := false
			var tableCreator string
			for creator, cmdData := range(humanTables) {
				tableID := d.TableID
				table, ok := tables[tableID]
				if !ok {
					panic("Table " + strconv.Itoa(tableID) + " does not exist.")
				}
				if len(table.Players) != 1 {
					panic("Table " + strconv.Itoa(tableID) + " has ",
						len(table.Players), " players")
				}

				// this guy has played with the table creator
				if _, played := playedWith[creator]; played {
					if !hasPlayedWith[creator][s.Username()] {
						panic("Error: Cannot find bot")
					}
					continue
				}

				// try to join the table
				commandTableJoin(s, &cmdData)
				if len(table.Players) == 2 {
					tableJoined := true
					tableCreator = creator
					hasPlayedWith[s.Username()][creator] = true
					hasPlayedWith[creator][s.Username()] = true
					break
				}
			}
			if tableJoined {
				fmt.Println(s.Username(), " joins ", tableCreator)
				delete(humanTables, tableCreator)
				continue
			}
		}

		// this guy will create a new table
		fmt.Println(s.Username(), " creates a new table")
		var cmdData CommandData
		cmdData.Variant = "No Variant"
		commandTableCreate(s, &cmdData)
		humanTables[s.Username()] = cmdData
	}

	fmt.Println("Log: #ErrorSession: ", len(errorSessions))
	fmt.Println("Log: #RemainingTable: ", len(humanTables))




	// var i int = 0
	// tableData := make([]CommandData, 0)
	// for _, s := range sessions {
	//	if i % 2 == 0 {
	//		fmt.Println(i, "th player online, create table")
	//		var cmdData CommandData
	//		cmdData.Variant = "No Variant"
	//		commandTableCreate(s, &cmdData)
	//		tableData = append(tableData, cmdData)
	//	} else {
	//		fmt.Println(i, "th player online, join table")
	//		commandTableJoin(s, &(tableData[len(tableData)-1]))
	//	}
	//	i += 1
	// }
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
