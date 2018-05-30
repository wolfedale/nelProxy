//
// Really simple API for Jenkins->Ansible tasks
//
// Pawel Grzesik, Engagehub 2018
// pawel.grzesik@gmail.com
//
// [How it works]
// Simply create Jenkins job with the specific JSON structure
// and call nelProxy with it.
// Ansible worker will call it as well and generate ansible
// playbook according to the JSON struct.
//
// Some examples of how to use it:
// Start server:
// go run main.go --server=127.0.0.1 --ssl=false --logs=./NelProxy.log --port=8080
//
// Start worker:
// go run main.go --server=127.0.0.1 --ssl=false --logs=./NelProxy.log --port=8080 --worker=true --inventory=EH2
//
// how to apply JSON
// curl -H "Content-Type: application/json" -d @test.json http://localhost:8080/task
//

package main

// Packages that we are using for the API purposes
import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// Task struct for ansible playbooks
type Task struct {
	ID        int     `json:"id"`
	Inventory string  `json:"inventory"`
	Command   Command `json:"command"`
}

// Command struct for ansible arguments
type Command struct {
	Playbook string `json:"playbook"`
	User     string `json:"user"`
	SU       bool   `json:"su"`
	Tags     Tags   `json:"tags"`
}

// Tags for ansible tags if we want to use
type Tags struct {
	Name []string `json:"name"`
}

// Tasks slice of Task struct
type Tasks []Task

// tasks variable Tasks type
var tasks Tasks

// tasks for workers
var jobs []Task

// currentID for generating ID in the struct
// default is 0
var currentID int

// menu foo
var menu ArgStruct

// ArgStruct for a nice menu cmd
type ArgStruct struct {
	SSLEnable bool
	SSLCert   string
	SSLKey    string
	Logs      string
	Server    string
	Port      string
	Worker    bool
	Inventory string
	Jformat   bool
}

func flagOptions() ArgStruct {
	// enable or disable SSL
	ssl := flag.Bool("ssl", false, "enable SSL")

	// path to the SSL Cert file
	sslcert := flag.String("ssl-cert", "", "path to a ssl cert file")

	// path to thr SSL Key file
	sslkey := flag.String("ssl-key", "", "path to a ssl key file")

	// path to the log file
	logs := flag.String("logs", "", "log file")

	// server IP
	server := flag.String("server", "", "server ip address")

	// server Port
	port := flag.String("port", "", "server port")

	// Am I worker or server?
	worker := flag.Bool("worker", false, "worker or server")

	// Production name or Inventory
	inventory := flag.String("inventory", "", "production name or inventory name")

	// JSON format on the output
	jformat := flag.Bool("jformat", false, "json format on the output")

	// parse all arg commands
	flag.Parse()

	// return them as struct
	return ArgStruct{
		*ssl,
		*sslcert,
		*sslkey,
		*logs,
		*server,
		*port,
		*worker,
		*inventory,
		*jformat}
}

func main() {
	/*
	   StrictSlash defines the trailing slash behavior for new routes. The initial
	   value is false. When true, if the route path is "/path/", accessing "/path"
	   will perform a redirect to theformer and vice versa. In other words, your
	   application will always see the path as specified in the route.
	   When false, if the route path is "/path", accessing "/path/" will not match
	   this route and vice versa.
	*/

	// Generate list of args for flag
	menu = flagOptions()

	// Check if we have server
	if menu.Server == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Here we are checking if we can create/open logfile
	logFile, err := os.OpenFile(menu.Logs, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	// Logging
	log.SetOutput(logFile)

	// Print my role
	if menu.Worker {
		// check if we are worker and have proper inventory name
		if menu.Inventory == "" {
			flag.PrintDefaults()
			os.Exit(1)
		}

		// executing worker
		err = Worker()
		if err != nil {
			log.Println("Error when executing Worker", err)
			os.Exit(2)
		}

	} else {
		// We are Server
		fmt.Println("Listening on port: ", menu.Port)
		// Creating new Route (using gorilla/max)
		router := NewRouter()

		// handlers is from the rogilla/handlers package
		loggedRouter := handlers.CombinedLoggingHandler(logFile, router)

		// Are we using PlainText or SSL mode?
		if menu.SSLEnable {
			log.Fatal(http.ListenAndServeTLS(":"+menu.Port, menu.SSLCert, menu.SSLKey, loggedRouter))
		} else {
			log.Fatal(http.ListenAndServe(":"+menu.Port, loggedRouter))
		}
	}
	logFile.Close()
}

// Worker foo
func Worker() error {
	// connecting to the API
	var data []byte

	response, err := http.Get("http://" + menu.Server + ":" + menu.Port + "/task")
	if err != nil {
		log.Printf("The HTTP request failed with error %s\n", err)
		return err
	}

	data, err = ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println("Cannot read data from Worker()", err)
		return err
	}

	// closing connection to the API
	defer response.Body.Close()

	// checking status code
	if response.StatusCode > 299 {
		log.Println("Status code:", response.StatusCode)
		os.Exit(2)
	}

	// parsing JSON
	err = json.Unmarshal(data, &jobs)
	if err != nil {
		log.Println("There was an error:", err)
	}

	// Execute Ansible
	err = Ansible()
	if err != nil {
		return err
	}

	// Return error
	return nil
}

// Ansible task
func Ansible() error {
	// iterate over all tasks
	for _, v := range jobs {

		// return jobs only for the proper production system
		if v.Inventory == menu.Inventory {

			// dealing with tags
			tags := setTags(v)

			// 1. Execute
			// it might be good to add error check here
			if menu.Jformat {
				AnsibleExecJSON(v, tags)
			} else {
				AnsibleExec(v, tags)
			}

			// 2. Drop
			err := AnsibleDrop(v, tags)
			if err != nil {
				return err
			}
		} else {
			log.Println("No tasks for the current production")
			fmt.Println("No tasks for the current production")
			os.Exit(0)
		}
	}
	return nil
}

// setTags
func setTags(v Task) string {
	var tags string
	for _, t := range v.Command.Tags.Name {
		tags += t + ","
	}
	if last := len(tags) - 1; last >= 0 && tags[last] == ',' {
		tags = tags[:last]
	}
	return tags
}

// AnsibleExec return text format
func AnsibleExec(v Task, t string) {
	// test with time sleep
	time.Sleep(5 * time.Second)
	if v.Command.SU {
		fmt.Println("ansible-playbook -i inventories/"+v.Inventory+"/hosts", v.Command.Playbook, "--ask-su-pass -u", v.Command.User, "--tags", t)
	} else {
		fmt.Println("ansible-playbook -i inventories/"+v.Inventory+"/hosts", v.Command.Playbook, "-u", v.Command.User, "--tags", t)
	}
}

// AnsibleExecJSON return json format
func AnsibleExecJSON(v Task, t string) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(b))
}

// AnsibleDrop foo
func AnsibleDrop(v Task, t string) error {
	// Create client
	client := &http.Client{}

	// convert Int to String
	myID := intToString(v.ID)

	// Create request
	req, err := http.NewRequest("DELETE", "http://"+menu.Server+":"+menu.Port+"/task/"+myID, nil)
	if err != nil {
		fmt.Println(err)
		return err
	}
	// Fetch Request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer resp.Body.Close()

	// Display Results
	log.Println("response Status : ", resp.Status)
	if resp.StatusCode != 200 {
		return err
	}

	return nil
}

// NewRouter function for all available routes
func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	// Routes
	router.HandleFunc("/task", GetTasks).Methods("GET")
	router.HandleFunc("/task", CreateTask).Methods("POST")
	router.HandleFunc("/task/{id}", GetTask).Methods("GET")
	router.HandleFunc("/task/{id}", DeleteTask).Methods("DELETE")

	return router
}

// GetTasks will return all tasks struct in the queue
func GetTasks(w http.ResponseWriter, r *http.Request) {
	// We want only JSON format
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// If there is nothing there yet return 500 to Jenkins
	if len(tasks) < 1 {
		// setup status code
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// setup http status code to 200
	w.WriteHeader(http.StatusOK)

	// Return tasks struct
	err := json.NewEncoder(w).Encode(tasks)
	if err != nil {
		log.Println(err)
	}
}

// GetTask return one specific task from the {id} in the path
func GetTask(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	intID := stringToInt(params["id"])
	for _, item := range tasks {
		if item.ID == intID {
			json.NewEncoder(w).Encode(item)
			return
		}
	}

	// return 404 if task do not exist
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

// CreateTask is simply creating new job for ansible
func CreateTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		http.Error(w, "Cannot create task, wrong JSON format", http.StatusInternalServerError)
		return
	}

	// We don't want to duplicate tasks for now.
	for _, item := range tasks {
		if (item.Inventory == task.Inventory) && (item.Command.Playbook == task.Command.Playbook) {
			http.Error(w, "Duplicated task", http.StatusInternalServerError)
			return
		}
	}

	// Every task need to have uniq number
	currentID++
	task.ID = currentID
	tasks = append(tasks, task)
	log.Println(task.ID, task.Inventory, task.Command.Playbook, task.Command.SU, task.Command.Tags, task.Command.User)
	http.Error(w, "Task has been created", http.StatusOK)
}

// DeleteTask is deleting job from the struct (queue)
func DeleteTask(w http.ResponseWriter, r *http.Request) {
	var found bool
	params := mux.Vars(r)
	intID := stringToInt(params["id"])

	for index, item := range tasks {
		if item.ID == intID {
			found = true
			tasks = append(tasks[:index], tasks[index+1:]...)
			break
		}
	}

	// If there is a job delete it and return 200, otherwise return 500 to Jenkins.
	if found {
		log.Println("Deleted task", intID)
		http.Error(w, "Task has been deleted", http.StatusOK)
	} else {
		http.Error(w, "No such task ID", http.StatusNotFound)
	}
}

// stringToInt is changing string to int
func stringToInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	return i
}

// intToString
func intToString(i int) string {
	t := strconv.Itoa(i)
	return t
}
