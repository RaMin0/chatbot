package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"time"

	cors "github.com/heppu/simple-cors"
	_ "github.com/joho/godotenv/autoload"
)

const (
	// WelcomeMessage A constant to hold the welcome message
	WelcomeMessage = "Welcome, what do you want to order?"
)

var (
	// sessions = {
	//   "uuid1" = Session{
	//     "history" = [
	//       "Message 1",
	//       "Message 2",
	//       "Message 3",
	//       ...
	//     ]
	//   },
	//   ...
	// }
	sessions = map[string]Session{}
)

type (
	// Session Holds info about a session
	Session map[string][]string

	// JSON Holds a JSON object
	JSON map[string]interface{}
)

func process(session Session, message string) (string, error) {
	// Make sure the message is unique in history
	for _, m := range session["history"] {
		if strings.EqualFold(m, message) {
			return "", fmt.Errorf("Duplicate message: %s", message)
		}
	}

	// Add the message in the parsed body to the messages in the session
	session["history"] = append(session["history"], message)

	// Form a sentence out of the history in the form Message 1, Message 2, and Message 3
	words := session["history"]
	l := len(words)
	wordsForSentence := make([]string, l)
	copy(wordsForSentence, words)
	if l > 1 {
		wordsForSentence[l-1] = "and " + wordsForSentence[l-1]
	}
	sentence := strings.Join(wordsForSentence, ", ")

	return fmt.Sprintf("So, you want %s! What else?", strings.ToLower(sentence)), nil
}

// withLog Wraps HandlerFuncs to log requests to Stdout
func withLog(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := httptest.NewRecorder()
		fn(c, r)
		log.Printf("[%d] %-4s %s\n", c.Code, r.Method, r.URL.Path)

		for k, v := range c.HeaderMap {
			w.Header()[k] = v
		}
		w.WriteHeader(c.Code)
		c.Body.WriteTo(w)
	}
}

// writeJSON Writes the JSON equivilant for data into ResponseWriter w
func writeJSON(w http.ResponseWriter, data JSON) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// handleWelcome Handles /welcome and responds with a welcome message and a generated UUID
func handleWelcome(w http.ResponseWriter, r *http.Request) {
	// Generate a UUID.
	hasher := md5.New()
	hasher.Write([]byte(strconv.FormatInt(time.Now().Unix(), 10)))
	uuid := hex.EncodeToString(hasher.Sum(nil))

	// Create a session for this UUID
	sessions[uuid] = Session{}

	// Write a JSON containg the welcome message and the generated UUID
	writeJSON(w, JSON{
		"uuid":    uuid,
		"message": WelcomeMessage,
	})
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	// Make sure only POST requests are handled
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST requests are allowed.", http.StatusMethodNotAllowed)
		return
	}

	// Make sure a UUID exists in the Authorization header
	uuid := r.Header.Get("Authorization")
	if uuid == "" {
		http.Error(w, "Missing or empty Authorization header.", http.StatusUnauthorized)
		return
	}

	// Make sure a session exists for the extracted UUID
	session, sessionFound := sessions[uuid]
	if !sessionFound {
		http.Error(w, fmt.Sprintf("No session found for: %v.", uuid), http.StatusUnauthorized)
		return
	}

	// Parse the JSON string in the body of the request
	data := JSON{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, fmt.Sprintf("Couldn't decode JSON: %v.", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Make sure a history key is defined in the session which points to a slice of strings
	_, historyFound := session["history"]
	if !historyFound {
		session["history"] = []string{}
	}

	// Make sure a message key is defined in the body of the request
	_, messageFound := data["message"]
	if !messageFound {
		http.Error(w, "Missing message key in body.", http.StatusBadRequest)
		return
	}

	// Process the received message
	message, err := process(session, data["message"].(string))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to process message: %v.", err), 422 /* http.StatusUnprocessableEntity */)
		return
	}

	// Write a JSON containg the processed response
	writeJSON(w, JSON{
		"message": message,
	})
}

// handle Handles /
func handle(w http.ResponseWriter, r *http.Request) {
	body :=
		"<!DOCTYPE html><html><head><title>Chatbot</title></head><body><pre style=\"font-family: monospace;\">\n" +
			"Available Routes:\n\n" +
			"  GET  /welcome -> handleWelcome\n" +
			"  POST /chat    -> handleChat\n" +
			"  GET  /        -> handle        (current)\n" +
			"</pre></body></html>"
	w.Header().Add("Content-Type", "text/html")
	fmt.Fprintln(w, body)
}

func main() {
	// HandleFuncs
	mux := http.NewServeMux()
	mux.HandleFunc("/welcome", withLog(handleWelcome))
	mux.HandleFunc("/chat", withLog(handleChat))
	mux.HandleFunc("/", withLog(handle))

	// Use the PORT environment variable
	port := os.Getenv("PORT")
	// Default to 3000 if no PORT environment variable was defined
	if port == "" {
		port = "3000"
	}

	// Start the server
	fmt.Printf("Listening on port %s...\n", port)
	log.Fatalln(http.ListenAndServe(":"+port, cors.CORS(mux)))
}
