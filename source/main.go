package main


import(
  "os"
  "log"
  "sync"
  "time"
  "bytes"
  "context"
  "syscall"
  "net/http"
  "os/signal"
  "encoding/json"
  "github.com/go-redis/redis"
  "github.com/satori/go.uuid"
  "github.com/gorilla/websocket"
)


const (
    writeTimeout = 10 * time.Second
    maxMessageSize = 512
    pongPeriod = 5 * time.Second
)


var (
    newline = []byte{'\n'}
    space = []byte{' '}
)


type Relay struct {
    Store *Store
    Redis *Redis
    Server *Server
}


type User struct {
  ID string
  Token string
  conn *websocket.Conn
  incoming chan *Event
  outgoing chan *Event
}


type Store struct {
  Users map[string]*User
  sync.Mutex
}


type Redis struct {
  Client *redis.Client
}


type Server struct {
  Address string
  Upgrader *websocket.Upgrader
  UserConnected chan *User
}


type Event struct {
    Type string `json:"event"`
    Channel string `json:"channel"`
    Data json.RawMessage `json:"data"`
}


// Create new Store Type
func NewStore () *Store {
  return &Store{
    Users: make(map[string]*User),
  }
}


func NewServer (address string) *Server {
  upgrader := &websocket.Upgrader{
    ReadBufferSize: 1024,
    WriteBufferSize: 1024,
    CheckOrigin: func (r *http.Request) bool {
      return true
    },
  }

  return &Server{
    Address: address,
    Upgrader: upgrader,
    UserConnected: make(chan *User),
  }
}


// Creates a new Redis Type with a Redis Client
func NewRedis (address string) *Redis {
  client := redis.NewClient(&redis.Options{
    Addr: address,
  })

  _, err := client.Ping().Result()

  if err != nil {
    panic(err)
  }

  return &Redis{
    Client: client,
  }
}


// Creates a new User Type
func NewUser (conn *websocket.Conn, token string) *User {
  return &User {
    ID: uuid.Must(uuid.NewV4()).String(),
    Token: token,
    conn: conn,
    incoming: make(chan *Event),
    outgoing: make(chan *Event),
  }
}


// Begins the relay process
func (relay *Relay) Start () {
  log.Println("[*] Starting HTTP Server")
  go relay.Server.Listen()

  for {
    select {
    case user := <- relay.Server.UserConnected:
      relay.HandleConnection(user)
      break
    }
  }
}


// Catch SIGHUP and SIGKILL for teardown processing
func (relay *Relay) Terminate () {
  signals := make(chan os.Signal, 1)
  signal.Notify(signals, syscall.SIGHUP, syscall.SIGKILL)

  go func () {
    <- signals
    os.Exit(0)
  }()
}


// Maps a new user connection to the store
func (relay *Relay) HandleConnection (user *User) {
  relay.Store.Lock()
  relay.Store.Users[user.ID] = user
  go user.Process(relay)
  log.Println("[*] Users:", len(relay.Store.Users))
  relay.Store.Unlock()
}


// Handles incoming event routing
func (relay *Relay) HandleIncoming (event *Event, user *User) {
  switch event.Type {
  case "subscribe":
    go user.Subscribe(relay, event.Channel)
    break
  case "message":
    relay.Publish(event)
  default:
    log.Printf("%s", event)
  }
}


// Publishes a message to a redis channel
func (relay *Relay) Publish (event *Event) {
  if event.Channel != "" {
    payload, err := json.Marshal(event)

    if err != nil {
      log.Println("JSON Encode Error", err)
      return
    }

    relay.Redis.Client.Publish(event.Channel, payload)
  }
}


// Starts the server listening for incoming connections
func (server *Server) Listen () {
  mux := http.NewServeMux()
  mux.HandleFunc("/connect", server.Upgrade)

  _server := &http.Server{
    Addr: server.Address,
    Handler: mux,
    ReadTimeout: time.Minute,
    WriteTimeout: time.Minute,
  }

  defer _server.Shutdown(context.Background())
  err := _server.ListenAndServe()
  if err != nil {
    panic(err)
  }
}


// Upgrades a HTTP request to persistant socket
func (server *Server) Upgrade (w http.ResponseWriter, r *http.Request) {
  params := r.URL.Query()

  if params["token"] == nil {
    log.Println("[*] Connect request made without token")
    return
  }

  conn, err := server.Upgrader.Upgrade(w, r, nil)

  if err != nil {
    panic(err)
  }

  token := string(params["token"][0])
  server.UserConnected <- NewUser(conn, token)
}


// Mainline processor for User connections
func (user *User) Process (relay *Relay) {
  go user.Heartbeat(relay)
  go user.Reader(relay)
  go user.Writer(relay)
  go user.Router(relay)
}


// Close instructs the relay to unmap the user
func (user *User) Close (relay *Relay) {
  relay.Store.Lock()
  user.conn.Close()
  delete(relay.Store.Users, user.ID)
  log.Println("[*] Users:", len(relay.Store.Users))
  relay.Store.Unlock()
}


// Heartbeat tests to see if user remains conencted through passive polling
func (user *User) Heartbeat (relay *Relay) {
  defer user.Close(relay)

  user.conn.SetPongHandler(func (string) error {
    user.conn.SetReadDeadline(time.Now().Add(pongPeriod))
    return nil
  })

  for {
    err := user.conn.WriteMessage(websocket.PingMessage, []byte("keep-alive"))
    if (websocket.IsCloseError(err)) {
      return
    }

    user.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
    time.Sleep(pongPeriod/2)
  }
}


// Reader accepts incoming byte stream from active socket
func (user *User) Reader (relay *Relay) {
  defer user.Close(relay)

  user.conn.SetReadDeadline(time.Now().Add(pongPeriod))
  user.conn.SetReadLimit(maxMessageSize)

  for {
    _, message, err := user.conn.ReadMessage()

    if err != nil {
      break
    }

    message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
    event := &Event{}
    json.Unmarshal(message, &event)
    user.incoming <- event
  }
}


// Writer buffers outgoing byte stream on active socket
func (user *User) Writer (relay *Relay) {
  defer user.Close(relay)

  for event := range user.outgoing {
    user.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
    writer, err := user.conn.NextWriter(websocket.TextMessage)

    if err != nil {
      return
    }

    payload, err := json.Marshal(event)

    if err != nil {
      log.Println("JSON Encode Error", err)
      return
    }

    writer.Write(payload)

    if err := writer.Close(); err != nil {
      return
    }
  }
}


// PubSub watches Users active redis subscriptions for incoming messages
func (user *User) Subscribe (relay *Relay, channel string) {
  psc := relay.Redis.Client.Subscribe(channel)
  defer psc.Close()

  for {
    message, err := psc.ReceiveMessage()

    if err != nil {
      panic(err)
    }

    event := &Event{}
    json.Unmarshal([]byte(message.Payload), &event)
    user.outgoing <- event
  }
}

// Router handles User data coming from various streams and pipes it
func (user *User) Router (relay *Relay) {
  for {
    select {
    case event := <-user.incoming:
      relay.HandleIncoming(event, user)
    }
  }
}


// entrypoint
func main () {
  log.Println("[*] Starting Relay")
  relay := &Relay{
    Store: NewStore(),
    Redis: NewRedis("websocket-redis:6379"),
    Server: NewServer("0.0.0.0:8080"),
  }

  done := make(chan bool)
  relay.Start()
  <- done
}
