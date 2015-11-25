package main

import (
  "github.com/spf13/viper"
  "github.com/mattes/migrate/migrate"
  "github.com/ant0ine/go-json-rest/rest"
  _ "github.com/lib/pq"
  "github.com/jinzhu/gorm"
  "fmt"
  "net/http"
  "log"
  "strconv"
  "time"
)
import _ "github.com/mattes/migrate/driver/postgres"

type DialogJson struct {
  Id                  uint8        `json:"id"`
  Name                string       `sql:"size:255" json:"name"`
  CreatedAt           time.Time    `json:"created_at"`
  UpdatedAt           time.Time    `json:"updated_at"`
  UserIds             string       `json:"user_ids"`
  LastMessage         string       `json:"last_message"`
  LastMessageID       int          `json:"last_message_id"`
  LastMessageUserID   int          `json:"last_message_user_id"`
  LastSeenMessageID   int          `json:"last_seen_message_id"`
}

type MessageJson struct {
  Id                  uint8        `json:"id"`
  Text                string       `json:"text"`
  CreatedAt           time.Time    `json:"created_at"`
  UserId              uint8        `json:"user_id"`
  DialogId            uint8        `json:"dialog_id"`
}

func main() {
  readConfig()
  applyMigrations()
  i := Impl{}
  i.connectToDb()
  i.startChat()
}

func readConfig() {
  viper.SetEnvPrefix("chat")
  viper.SetDefault("database_url", "postgres:///chat_development?sslmode=disable")
  viper.SetDefault("bind_address", "localhost:8080")
  viper.AutomaticEnv()
  viper.BindEnv("database_url")
  viper.BindEnv("bind_address")
}

func applyMigrations() {
  allErrors, ok := migrate.UpSync(viper.GetString("database_url"), "./migrations")
  fmt.Println("Database: ", viper.GetString("database_url"))
  if !ok {
    fmt.Println("Migratinos failed")
    fmt.Println("driver: ", viper.GetString("database_url"))
    fmt.Println("Errors: ", allErrors)
  }
}

func (i *Impl) connectToDb() {
  var err error
  i.DB, err = gorm.Open("postgres", viper.GetString("database_url"))
  if err != nil {
      log.Fatalf("Got error when connect database, the error is '%v'", err)
  }
  i.DB.LogMode(true)
}

type Impl struct {
  DB gorm.DB
}

func (i *Impl) startChat() {
  api := rest.NewApi()
  api.Use(rest.DefaultDevStack...)
  router, err := rest.MakeRouter(
    rest.Get("/", Index),
    rest.Get("/users/:user_id/dialogs", i.DialogIndex),
    rest.Get("/users/:user_id/dialogs/:dialog_id", i.DialogShow),
    rest.Post("/users/:user_id/dialogs", DialogCreate),
    rest.Post("/users/:user_id/dialogs/:dialog_id/messages", MessageCreate),
  )
  if err != nil {
    log.Fatal(err)
  }
  api.SetApp(router)
  fmt.Println("address: ", viper.GetString("bind_address"))
  log.Fatal(http.ListenAndServe(viper.GetString("bind_address"), api.MakeHandler()))
}

func Index(w rest.ResponseWriter, r *rest.Request) {
  w.WriteJson("Welcome!\n")
}

func (i *Impl) DialogIndex(w rest.ResponseWriter, r *rest.Request) {
  userId := r.PathParam("user_id")
  page, err := strconv.Atoi(r.FormValue("page"))
  offset := 0
  if err == nil {
    offset = (page - 1) * 10
  }

  dialogs := []DialogJson{}
  i.DB.Raw(`
    SELECT c.*, array_agg(du.user_id) AS user_ids
    FROM
      (SELECT
        dialogs.id AS id,
        dialogs.name AS name,
        dialogs.created_at AS created_at,
        dialogs.updated_at AS updated_at,
        messages.text AS last_message,
        messages.user_id AS last_message_user_id,
    	dialog_users.last_seen_message_id AS last_seen_message_id
      FROM dialogs
      JOIN messages ON messages.id = dialogs.last_message_id
      JOIN dialog_users ON dialog_users.dialog_id = dialogs.id
      WHERE dialog_users.user_id = ?
      ORDER BY dialogs.last_message_id DESC
      ) c
    JOIN dialog_users du ON c.id = du.dialog_id
    GROUP BY
      c.id,
      c.name,
      c.created_at,
      c.updated_at,
      c.last_message,
      c.last_message_user_id,
      c.last_seen_message_id
    LIMIT 10
    OFFSET ?
  `, userId, offset).Find(&dialogs)
  w.WriteJson(&dialogs)
}

func (i *Impl) DialogShow(w rest.ResponseWriter, r *rest.Request) {
  userId := r.PathParam("user_id")
  dialogId := r.PathParam("dialog_id")
  page, err := strconv.Atoi(r.FormValue("page"))
  offset := 0
  if err == nil {
    offset = (page - 1) * 10
  }
  messages := []MessageJson{}
  i.DB.Raw(`
    SELECT * FROM messages
    WHERE messages.dialog_id = ?
    ORDER BY messages.id DESC
    LIMIT 10
    OFFSET ?
  `, dialogId, offset).Find(&messages)

  last_message_id := 0
  i.DB.Raw("SELECT last_message_id FROM dialogs WHERE id = ?", dialogId).Row().Scan(&last_message_id)
  i.DB.Exec("UPDATE dialog_users SET last_seen_message_id = ? WHERE dialog_id = ? AND user_id = ?", last_message_id, dialogId, userId)

  w.WriteJson(&messages)
}

func DialogCreate(w rest.ResponseWriter, r *rest.Request) {
  // fmt.Fprintf(w, "create, %s!\n", ps.ByName("user_id"))
}

func MessageCreate(w rest.ResponseWriter, r *rest.Request) {
  // fmt.Fprintf(w, "create, %s!\n", ps.ByName("dialog_id"))
}
