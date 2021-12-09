package main

import (
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	//log "github.com/sirupsen/logrus"
	//"github.com/gorilla/mux"
)

var getMyKeyRe *regexp.Regexp = regexp.MustCompile("/api/v1/my/keys/([^/]+)") // Regex for get operation

const DEFAULT_PERSISTANCE_INTERVAL = 300 // in seconds

type APIOPERATION int // API operation enum

// Operation enum
const (
	CREATE    APIOPERATION = 0
	GET                    = 1
	DELETEALL              = 2
)

// ServerX interface handles create, get, delete all API request
// Tags request and response with header value x-request-id, if a valid requets id exists in request header uses the same value in response
// If cannot find a valid request id then creates a new uuid
// Starts an operation listener to handle each API operation, works on a shared dictionary using channels
// It listens timer tick event from persistance object, when timer ticks ServerX receives the event from a channel then sends the current dict to persistance object via channel
// TODO: authentication, authorization
// TODO: validate request against swagger/openapi3 document (json schemas)
// TODO: cosider implementing rate limiting
// TODO: consider threat protection
// TODO: enhanced logging using levels
// TODO: configuration per env (staging, prod)
type ServerX interface {
	Handle(w http.ResponseWriter, r *http.Request)
	Route(w http.ResponseWriter, r *http.Request)
	Tag(w http.ResponseWriter, r *http.Request)
	/* Listen for events */
	StartApiOperationListener()
	/* Endpoint handlers */
	Create(w http.ResponseWriter, r *http.Request)
	Get(w http.ResponseWriter, r *http.Request)
	DeleteAll(w http.ResponseWriter, r *http.Request)
}

// ServiceX holds the shared dictionary
// Create, get, delete all operations on the dictionary are performed in go routine as concurrent
// operationChan is fed by create, get, delete all endpoints
// persistance object performs file system operations
// TODO: When multiple instances run, changes (writes) on the dict must be synchronized to other instances (in a container environment)
// TODO: Synch could be done manually, using rest, message broker, or a distributed memory cache like redis, memcache, hazelcast
type ServiceX struct {
	dict          map[string]string
	operationChan chan ApiOperation
	persistance   *FSPersistance
}

// NewService creates a service with an optional interval value, default internal is defined as DEFAULT_PERSISTANCE_INTERVAL
// Initializes dict, operationChan, and persistance
// peristance checks the file system for a previosly persisted dict
// Starts a go routine to listen API operations
// TODO: persistance.RestoreFromPersistance should be considered in a container environment; the current insrance may try to get latest dict from other instances
// TODO: if cannot get any data from other instances it could try to get latest data from files system as a last option
func NewService(args ...interface{}) *ServiceX {
	interval := DEFAULT_PERSISTANCE_INTERVAL
	for _, arg := range args {
		switch t := arg.(type) {
		case int:
			interval = t
		default:
			panic("Unknown argument")
		}
	}

	var s ServiceX
	s.dict = make(map[string]string)
	s.operationChan = make(chan ApiOperation, 100) // buffered channel
	s.persistance = NewPersistance(interval)
	// read backup if exists
	dict, err := s.persistance.RestoreFromPersistance()
	if err == nil {
		s.dict = dict
		log.Printf("INFO Data recovered from tmp directory. dict.len:%v \r\n", len(s.dict))
	}
	s.StartApiOperationListener()
	return &s
}

// ApiOperation is data stucture for communication
// !!! Share Memory By Communicating !!!
// oper is an enumaration for CREATE, GET, DELETEALL
// key and value attributes are for receiving data from endpoint handlers
// respData and ack is used to give response and ack to endpoint listeners
type ApiOperation struct {
	oper     APIOPERATION
	key      string
	value    string
	respData chan map[string]string
	ack      chan bool
}

// NewApiOperation initializes an ApiOperation, Endpoint handlers will feed operationChan and get response via ack and respData
func NewApiOperation() *ApiOperation {
	var a ApiOperation
	a.respData = make(chan map[string]string, 1)
	a.ack = make(chan bool)
	return &a
}

// StartApiOperationListener waits for events from endpoint handlers and persistance.timer in a go routine
// the function works on shared dictionary in a go routine, receives data over channel and respond via channel
func (s *ServiceX) StartApiOperationListener() {
	go func() {
		for {
			select {
			case t := <-s.persistance.ticker.C:
				// Got timer tick from persistance
				//if len(s.dict) > 0 {
				log.Printf("DEBUG Peristance timer tick at:%v. Send current dict to persistance. Dict.len:%v", t, len(s.dict))
				s.persistance.persistanceChan <- s.dict
				//}
			case apiOp := <-s.operationChan:
				// Get event from endpoints. Process the event by type
				switch apiOp.oper {
				case CREATE:
					// Add a new key value to dictionary, then respond
					s.dict[apiOp.key] = apiOp.value
					apiOp.ack <- true
				case GET:
					// Find the value by given key and respond
					if _, ok := s.dict[apiOp.key]; ok {
						apiOp.respData <- map[string]string{apiOp.key: s.dict[apiOp.key]}
						apiOp.ack <- true
					} else {
						apiOp.ack <- false
					}
				case DELETEALL:
					s.dict = make(map[string]string)
					apiOp.ack <- true
				default:
					apiOp.ack <- false
				}
			}
		}
	}()
}

// Create API operation creates a new key value in dictionary
// TODO: Create and Update could be seperated using POST and PUT methods
// @Summary Create a new pair or update existing
// @Description create
// @Tags GoApp
// @Accept json
// @Param pair body map[string]string true "Pair"
// @Success 201
// @Failure 500,415,405,404
// @Router /my/keys [post]
func (s *ServiceX) Create(w http.ResponseWriter, r *http.Request) {
	result := make(map[string]string)
	var _err = json.NewDecoder(r.Body).Decode(&result)
	if _err != nil {
		http.Error(w, _err.Error(), http.StatusBadRequest)
		return
	}

	// Communicate with listener over channel
	ao := NewApiOperation()
	ao.oper = CREATE
	ao.key = reflect.ValueOf(result).MapKeys()[0].String()
	ao.value = result[ao.key]
	s.operationChan <- *ao

	// get the response from listener
	if ack := <-ao.ack; ack {
		w.WriteHeader(http.StatusCreated)
		log.Printf("INFO Create completed. RequestId: %v\r\n", w.Header().Get("x-request-id"))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("ERROR Create failed. RequestId: %v, err:%v\r\n", w.Header().Get("x-request-id"), _err.Error())
	}
}

// Get API operation gets given key and value pair from dictionary by given key as path variable
// @Summary Get pair
// @Description get pair
// @Tags GoApp
// @Param key path string true "key"
// @Success 200 {string} resp
// @Failure 500,415,405,404
// @Router /my/keys/{key} [get]
func (s *ServiceX) Get(w http.ResponseWriter, r *http.Request) {
	ss := strings.Split(r.URL.Path, "/")

	// Communicate with listener over channel
	ao := NewApiOperation()
	ao.oper = GET
	ao.key = ss[len(ss)-1]
	s.operationChan <- *ao

	var resp map[string]string
	// get the response from listener
	if ack := <-ao.ack; ack {
		resp = <-ao.respData
		var jsonStr, _err = json.Marshal(resp)
		if _err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("ERROR Get failed. RequestId: %v, err:%v\r\n", w.Header().Get("x-request-id"), _err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		log.Printf("INFO Get completed. RequestId: %v\r\n", w.Header().Get("x-request-id"))
		w.Write(jsonStr)
	} else {
		w.WriteHeader(http.StatusNotFound)
		log.Printf("WARN Get completed. RequestId: %v\r\n", w.Header().Get("x-request-id"))
	}
}

// DeleteAll API operation deletes all data in the dictionary
// @Summary Delete All
// @Description delete all
// @Tags GoApp
// @Success 204
// @Failure 500,415,405,404
// @Router /my/keys [delete]
func (s *ServiceX) DeleteAll(w http.ResponseWriter, r *http.Request) {
	// Communicate with listener over channel
	ao := NewApiOperation()
	ao.oper = DELETEALL
	s.operationChan <- *ao

	// get the response from listener
	if ack := <-ao.ack; ack {
		w.WriteHeader(http.StatusNoContent)
		log.Printf("INFO DeleteAll completed. RequestId: %v\r\n", w.Header().Get("x-request-id"))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("ERROR DeleteAll failed. RequestId: %v\r\n", w.Header().Get("x-request-id"))
	}
}

// Handle is fisrt point that any endpoint handled. Tags request and response. Checks for the http method
func (s *ServiceX) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.Tag(w, r)

	//log.Printf("INFO Request(%v) received. RemoteAddr:%v, [%v] %v, referrer:%v, TLS.version:%v, TLS.cipher:%v \r\n", r.Header.Get("x-request-id"), r.RemoteAddr, r.Method, r.RequestURI, r.Referer(), r.TLS.Version, r.TLS.CipherSuite)
	log.Printf("INFO Pid:%v Request(%v) received. RemoteAddr:%v, [%v] %v x-forwarded-for:%v, x-forwarded-port:%v, x-real-ip:%v, x-forwarded-host:%v\r\n", syscall.Getpid(), r.Header.Get("x-request-id"), r.RemoteAddr, r.Method, r.URL.Path, r.Header.Get("x-forwarded-for"), r.Header.Get("x-forwarded-port"), r.Header.Get("x-real-ip"), r.Header.Get("x-forwarded-host"))

	// set return content-type
	w.Header().Set("Content-Type", "application/json")

	// check request content-type
	if r.Header.Get("Content-type") != "application/json" {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		log.Printf("ERROR UnsupportedMediaType. RequestId: %v\r\n", w.Header().Get("x-request-id"))
		return
	}

	// check for allowed http methods
	switch r.Method {
	case "POST", "PUT", "GET", "DELETE":
		s.Route(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		log.Printf("ERROR MethodNotAllowed. RequestId: %v\r\n", w.Header().Get("x-request-id"))
	}
	log.Printf("INFO Request '%v' completed in %v.\r\n", w.Header().Get("x-request-id"), time.Since(start).Truncate(time.Millisecond))
}

// Route is the router, checks for endpoints and calls corresponding API operation
func (s *ServiceX) Route(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "POST" && r.URL.Path == "/api/v1/my/keys":
		s.Create(w, r)
	case r.Method == "GET" && getMyKeyRe.MatchString(r.URL.Path):
		s.Get(w, r)
	case r.Method == "DELETE" && r.URL.Path == "/api/v1/my/keys":
		s.DeleteAll(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		log.Printf("ERROR NotFound. RequestId: %v\r\n", w.Header().Get("x-request-id"))
	}
}

// Tag tags request and response with valid uuid
func (s *ServiceX) Tag(w http.ResponseWriter, r *http.Request) {
	_, err := uuid.Parse(r.Header.Get("x-request-id"))
	if err == nil {
		w.Header().Set("x-request-id", r.Header.Get("x-request-id")) // give the same value in http response header
	} else {
		// generate new uuid
		requestId := uuid.New().String()
		r.Header.Set("x-request-id", requestId)
		w.Header().Set("x-request-id", requestId)
	}
}
