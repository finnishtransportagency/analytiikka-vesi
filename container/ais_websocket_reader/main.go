package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/gorilla/websocket"
)


var region_name = "eu-west-1"
var parameter_name = "api-ais-websocket-reader"

//var bucketRegion = "eu-west-1"
//var s3bucket = "aistestbucket"
var s3bucket = ""

var rawList = rawStructList{}
var parsedList = parsedStructList{}
var lock = sync.Mutex{}
var parsedlock = sync.Mutex{}
var uusername = "user"
var upassword = "pass"
var messagelimit = 30000
var apihost = "host"
var apipath = "path"
var delimiter = "$"
var pair_delimiter = "|"

const errorfloat = 1000000

/*****Data from websocket*******/

/*******************************/

/***** raw structure for JSON and internal objects*********/

type rawStructList struct {
	Messages struct {
		MessagesWithStamp []rawMessage `json:"Messages"`
	} `json:"AISRawMessages"`
}

type rawMessage struct {
	ProcessTimestamp int      `json:"processTimestamp"`
	Message          []string `json:"message"`
}

/*******************************/

/***** Parsed  structure  and internal objects*********/

type parsedStructList struct {
	Messages struct {
		ParsedStructMessages []ParsedMessage `json:"Messages"`
	} `json:"AISParsedmessages"`
}

/*
ParsedMessage contains all known attributes of AIS data
*/
type ParsedMessage struct {
	//Remember these have to start with uppercase or go wont export them to JSON
	HexState    *string         `json:"Communication state in hex,omitempty"`
	SDimension  *ShipDimensions `json:"Dimension of ship reference for position,omitempty"`
	ETA         *int            `json:"ETA [MMDDHHmm],omitempty"`
	Longitude   *float32        `json:"Longitude,omitempty"`
	Latitude    *float32        `json:"Latitude,omitempty"`
	Name        *string         `json:"Name,omitempty"`
	Destination *string         `json:"Destination,omitempty"`
	Vid         *string         `json:"Vendor ID in hex,omitempty"`
	//Following sseem to be optional integer flags
	Asensor          *int `json:"Altitude sensor,omitempty"`
	Amodeflag        *int `json:"Assigned mode flag,omitempty"`
	CBBandFlag       *int `json:"Class B band flag,omitempty"`
	CBDisFlag        *int `json:"Class B display flag,omitempty"`
	CBDSCFlag        *int `json:"Class B DSC flag,omitempty"`
	CBMessageFlag    *int `json:"Class B Message 22 flag,omitempty"`
	CBUnitFlag       *int `json:"Class B unit flag,omitempty,omitempty"`
	ComStateSelector *int `json:"Communication state selector flag,omitempty"`
	DTE              *int `json:"DTE,omitempty"`
	Nstatus          *int `json:"Navigational status,omitempty"`
	Pnumber          *int `json:"Part number,omitempty"`
	PosAccuracy      *int `json:"Position accuracy,omitempty"`
	PosLatency       *int `json:"Position latency,omitempty"`
	RFlag            *int `json:"RAIM-flag,omitempty"`
	SmanI            *int `json:"Special manoeuvre indicator,omitempty"`
	PFDT             *int `json:"Type of electronic position fixing device,omitempty"`
	TSG              *int `json:"Type of ship and cargo type,omitempty"`
	//not added to matcher and not documented
	Callsign     *string  `json:"Call sign,omitempty"`
	AISVersion   *int     `json:"AIS Version,omitempty"`
	MessageID    *int     `json:"Message ID,omitempty"`
	Repeati      *int     `json:"Repeat indicator,omitempty"`
	Spare        *int     `json:"Spare,omitempty"`
	Spare2       *int     `json:"Spare (2),omitempty"`
	IMONumber    *int64   `json:"IMO number,omitempty"`
	EtimeStamp   *int64   `json:"Ext_timestamp,omitempty"`
	TimeStamp    *int64   `json:"Time stamp,omitempty"`
	UID          *int64   `json:"User ID,omitempty"`
	TrueHeading  *float32 `json:"True heading,omitempty"`
	COG          *float32 `json:"COG,omitempty"`
	SOG          *float32 `json:"SOG,omitempty"`
	TurnRate     *int     `json:"Rate of Turn ROTAIS,omitempty"`
	MPSD         *float32 `json:"Maximum present static draught,omitempty"`
	GNSSAltitude *int     `json:"Altitude (GNSS),omitempty"`
}

/*
ShipDimensions contains A,B,C,D dimensions from AIS data
*/
type ShipDimensions struct {
	ADim int `json:"A"`
	BDim int `json:"B"`
	CDim int `json:"C"`
	DDim int `json:"D"`
}

/*******************************/

type aISData struct {
	Data struct {
		Raw    []string `json:"raw"`
		Parsed string   `json:"parsed"`
	} `json:"data"`
}

/*
StoragePathAndFileNaming generates S3 filepath for parsed and raw data depending on bool value. Path also includes date format and filename includes current time in nanoseconds, random hex and AIS.JSON.GZ since data will be compressed
*/
func StoragePathAndFileNaming(parsed bool) string {
	var s3prefix string
	currentTime := time.Now()
	var dateSaltprefix = currentTime.Format("2006/01/02") + "/" + strconv.FormatInt(time.Now().UnixNano(), 10) + randomHex() + "AIS.json.gz"
	if parsed == true {
		s3prefix = "parsed/" + dateSaltprefix
	} else {
		s3prefix = "raw/" + dateSaltprefix
	}
	return s3prefix
}

func compressGZ(w io.Writer, data []byte) error {
	gzw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	defer gzw.Close()
	gzw.Write(data)
	gzw.Flush()
	return err
}

/*
initDumpToS3 initilizes variables, dumps data to byte array and compresses it for S3dump method uses
*/
func initDumpToS3(parsed bool) {
	var buf bytes.Buffer
	var JsonbyteDump []byte
	var err error
	if parsed {
		JsonbyteDump, err = json.Marshal(parsedList)
	} else {
		JsonbyteDump, err = json.Marshal(rawList)
	}
	if err != nil {
		log.Println(err)
	}
	var storagePath = StoragePathAndFileNaming(parsed) // data is not parsed type so false
	gzErr := compressGZ(&buf, JsonbyteDump)
	if gzErr != nil {
		log.Println("Could not dump gunzip", gzErr)
	} else {
		go dumpToS3(buf, storagePath)
	}
}

/*
dumpToS3 literally uploads data to S3 with given parameters
*/
func dumpToS3(buf bytes.Buffer, path string) {
	log.Println("Starting to dump")
	s, err := session.NewSession(&aws.Config{Region: aws.String(region_name)})
	uploader := s3manager.NewUploader(s)
	if err != nil {
		log.Println(err)
	}
	_, errs := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s3bucket),
		Key:    aws.String(path),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	if errs != nil {
		log.Println(errs)
	}
}

/*
	1) get data to struct so that we have raw and parsed ones
	2) extract data from struct
	3.1 add raw data to list
	3.2 add parsed data to list
	3.3 when list size reaces defined size dump data to S3 raw in /raw "folder" & parsed in "parsed folder"
*/
func messageprosessor(byteMessage []byte, test bool) {
	structuredData := stringMessageToStruct(byteMessage)         // 1
	rawData, parsedData := extractDataFromStruct(structuredData) // 2
	if len(rawData) > 0 {
		go rawDataprocessLogic(rawData, test) //3.1, 3.3
	}
	if len(parsedData) > 0 {
		go parsedDataprocessLogic(parsedData, test) // 3.2, 3.3
	}
}

func getrawStructList() rawStructList {
	return rawList
}

func extractDataFromStruct(ais aISData) ([]string, string) {
	return ais.Data.Raw, ais.Data.Parsed
}

func stringMessageToStruct(byteMessage []byte) aISData {
	strucMessage := aISData{}
	json.Unmarshal(byteMessage, &strucMessage)
	return strucMessage
}

/**
Creates messageobject holding array of messages
*/
func rawMessageObjectCreator(rawData []string) rawMessage {
	var newMessage rawMessage
	for _, rawMessage := range rawData {
		newMessage.Message = append(newMessage.Message, rawMessage)
	}
	newMessage.ProcessTimestamp = int(time.Now().Unix())
	return newMessage
}

/**
raw data is added to slice (golang array) inside lock so only on thread can add content to list at the time. If list has reached required size slice will be passed to S3 dumping function
*/

func rawDataprocessLogic(rawData []string, test bool) bool {
	lock.Lock()
	defer lock.Unlock()
	var newStrucutredRawMessage = rawMessageObjectCreator(rawData)
	rawList.Messages.MessagesWithStamp = append(rawList.Messages.MessagesWithStamp, newStrucutredRawMessage)
	if len(rawList.Messages.MessagesWithStamp) >= messagelimit {
		var parsed = false
		if !test {
			log.Println("initing Raw dumping to s3")
			initDumpToS3(parsed)
		}
		rawList = rawStructList{}
		return true //tells that list was dumped to s3
	}
	return false
}

func parsedDataprocessLogic(parsedData string, test bool) bool {
	parsedlock.Lock()
	defer parsedlock.Unlock()
	var newStrucutredParsedMessage = ParsedMessageObjectConverter(parsedData)
	parsedList.Messages.ParsedStructMessages = append(parsedList.Messages.ParsedStructMessages, newStrucutredParsedMessage)
	if len(parsedList.Messages.ParsedStructMessages) >= messagelimit {
		var parsed = true
		if !test {
			log.Println("initing Parsed dumping to s3")
			initDumpToS3(parsed)
		}
		parsedList = parsedStructList{}
		return true
	}
	return false
}

/*
ParsedMessageObjectConverter converts string to Parsedmessage object
*/
func ParsedMessageObjectConverter(parsedData string) ParsedMessage {
	var splittedData = strings.Split(parsedData, pair_delimiter)
	var newParsedMessage = ParsedMessage{}
	newParsedMessage = matchingloop(newParsedMessage, splittedData, &parsedData)
	return newParsedMessage
}

func getsplittedFloatValue(keyvaluepair string) (float32, error) {
	var splittedpair = strings.Split(keyvaluepair, delimiter)
	if len(splittedpair) != 2 {
		return 0, errors.New("Failed to parse float value")
	}
	longstring := splittedpair[1]
	longval, err := strconv.ParseFloat(longstring, 32)
	if err != nil {
		log.Println("failed to convert float")
		return 0, errors.New("Fatal: failed to parse float value")
	}
	return float32(longval), nil

}

func getsplittedStringValue(keyvaluepair string) (string, error) {
	var splittedpair = strings.SplitN(keyvaluepair, delimiter, 3)
	if len(splittedpair[1]) > 0 {
		return splittedpair[1], nil
	}
	return "", errors.New("Fatal:Failed to parse String " + keyvaluepair)

}

func getsplittedStringKey(keyvaluepair string) (string, error) {
	var splittedpair = strings.SplitN(keyvaluepair, delimiter, 2)
	if len(splittedpair[0]) > 0 {
		return splittedpair[0], nil
	}
	return "", errors.New("Fatal:Failed to parse key-value" + keyvaluepair)
}

//Call sign
//maybe if _, err := rand.Read(bytes); err != nil {
//// remember try catch this
func matchingloop(newMessage ParsedMessage, splitted []string, originalMessage *string) ParsedMessage {
	for _, keyvaluepair := range splitted {

		var key, errk = getsplittedStringKey(keyvaluepair)
		if errk != nil {
			log.Println("Fatal error: Content broken json ", keyvaluepair)
			log.Println(*originalMessage)
			break
		}
		switch key {
		case "Communication state in hex":
			sValue, err := getsplittedStringValue(keyvaluepair)
			if err != nil {
				log.Println("state in hex ", err)
				log.Println(*originalMessage)
			} else {
				newMessage.HexState = &sValue
			}
		case "Name":
			sValue, err := getsplittedStringValue(keyvaluepair)
			if err != nil {
				log.Println(err)
				log.Println(*originalMessage)
			} else {
				newMessage.Name = &sValue
			}
		case "Call sign":
			sValue, err := getsplittedStringValue(keyvaluepair)
			if err != nil {
				log.Println(err)
				log.Println(*originalMessage)
			} else {
				newMessage.Callsign = &sValue
			}
		case "Destination":
			sValue, err := getsplittedStringValue(keyvaluepair)
			if err != nil {
				log.Println(err)
				log.Println(*originalMessage)
			} else {
				newMessage.Destination = &sValue
			}
		case "Vendor ID in hex":
			sValue, err := getsplittedStringValue(keyvaluepair)
			if err != nil {
				log.Println(err)
				log.Println(*originalMessage)
			} else {
				newMessage.Vid = &sValue
			}
		case "Dimension of ship/reference for position":
			sDime, err := GetDimensions(keyvaluepair)
			if err != nil {
				log.Println(err)
				log.Println(*originalMessage)
			} else {
				newMessage.SDimension = &sDime
			}
		case "Longitude":
			floater, err := getsplittedFloatValue(keyvaluepair)
			if err != nil {
				log.Println("Longitude ", err)
				log.Println(*originalMessage)
			} else {
				newMessage.Longitude = &floater
			}
		case "Latitude":
			floater, err := getsplittedFloatValue(keyvaluepair)
			if err != nil {
				log.Println(" Latitude", err)
				log.Println(*originalMessage)
			} else {
				newMessage.Latitude = &floater
			}
		case "True heading":
			floater, err := getsplittedFloatValue(keyvaluepair)
			if err != nil {
				log.Println("True heading", err)
				log.Println(*originalMessage)
			} else {
				newMessage.TrueHeading = &floater
			}
		case "COG":
			floater, err := getsplittedFloatValue(keyvaluepair)
			if err != nil {
				log.Println("COG", err)
				log.Println(*originalMessage)
			} else {
				newMessage.COG = &floater
			}
		case "SOG":
			floater, err := getsplittedFloatValue(keyvaluepair)
			if err != nil {
				log.Println("SOG", err)
				log.Println(*originalMessage)
			} else {
				newMessage.SOG = &floater
			}
		case "Rate of turn ROTAIS":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal: Rate of turn ROTAIS:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.TurnRate = &iValue
			}
		case "Maximum present static draught":
			floater, err := getsplittedFloatValue(keyvaluepair)
			if err != nil {
				log.Println("in Rate of turn ROTAIS", err)
				log.Println(*originalMessage)
			} else {
				newMessage.MPSD = &floater
			}

		case "ETA [MMDDHHmm]":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.ETA = &iValue
			}
		case "Altitude sensor":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.Asensor = &iValue
			}
		case "Assigned mode flag":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.Amodeflag = &iValue
			}
		case "Class B band flag":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.CBBandFlag = &iValue
			}
		case "Class B DSC flag":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.CBDSCFlag = &iValue
			}
		case "Class B display flag":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.CBDisFlag = &iValue
			}
		case "Class B Message 22 flag":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.CBMessageFlag = &iValue
			}
		case "Class B unit flag":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.CBUnitFlag = &iValue
			}
		case "Communication state selector flag":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.ComStateSelector = &iValue
			}
		case "DTE":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {

				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.DTE = &iValue
			}
		case "Navigational status":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.Nstatus = &iValue
			}
		case "Part number":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.Pnumber = &iValue
			}
		case "Position accuracy":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.PosAccuracy = &iValue
			}
		case "Position latency":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.PosLatency = &iValue
			}
		case "RAIM-flag":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.RFlag = &iValue
			}
		case "Special manoeuvre indicator":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.SmanI = &iValue
			}
		case "Type of electronic position fixing device":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.PFDT = &iValue
			}
		case "AIS version indicator":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.AISVersion = &iValue
			}

		case "Message ID":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.MessageID = &iValue
			}

		case "Repeat indicator":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.Repeati = &iValue
			}
		case "Spare":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.Spare = &iValue
			}
		case "Spare (2)":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.Spare2 = &iValue
			}
		case "Type of ship and cargo type":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.TSG = &iValue
			}
		case "IMO number":
			iValue, err := getsplittedInteger64Value(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.IMONumber = &iValue
			}
		case "Ext_timestamp":
			iValue, err := getsplittedInteger64Value(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.EtimeStamp = &iValue
			}
		case "Time stamp":
			iValue, err := getsplittedInteger64Value(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.TimeStamp = &iValue
			}
		case "User ID":
			iValue, err := getsplittedInteger64Value(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.UID = &iValue
			}
		case "Altitude (GNSS)":
			iValue, err := getsplittedIntegerValue(keyvaluepair)
			if err != nil {
				log.Println("Fatal:", err)
				log.Println(*originalMessage)
			} else {
				newMessage.GNSSAltitude = &iValue
			}
		default:
			log.Println("FATAL: Could not match value-key-pair", keyvaluepair)
			log.Println(*originalMessage)
		}
	}
	return newMessage
}

//AIS version indicator

/*
GetDimensions splits keyvalue pair that contains a,b,c,d info of the vessel
second split splits a,b,c,d to their own pairs
which we try to match and import to ships dimension structure and return it
*/
func GetDimensions(keyvaluepair string) (ShipDimensions, error) {
	var newDimensions = ShipDimensions{}
	var splittedpair = strings.Split(keyvaluepair, delimiter)
	if len(splittedpair) != 2 {
		return newDimensions, errors.New("failed to split dimension keyvalue pair")
	}
	var splittedValues = strings.Split(splittedpair[1], ",")
	if len(splittedValues) < 1 {

		return newDimensions, errors.New("failed to split dimension keyvalue pair")
	}
	for _, dim := range splittedValues {
		if strings.Contains(dim, "A") {
			integerValue, err := givedimensionvalue(dim)
			if err != nil {
				log.Println("Fatal could not parse A", dim, err)
				return newDimensions, err
			}
			newDimensions.ADim = integerValue

		} else if strings.Contains(dim, "B") {
			integerValue, err := givedimensionvalue(dim)
			if err != nil {
				log.Println("Fatal could not parse B", dim, err)
				return newDimensions, err
			}
			newDimensions.BDim = integerValue

		} else if strings.Contains(dim, "C") {
			integerValue, err := givedimensionvalue(dim)
			if err != nil {
				log.Println("Fatal could not parse C", dim, err)
				return newDimensions, err
			}
			newDimensions.CDim = integerValue

		} else if strings.Contains(dim, "D") {
			integerValue, err := givedimensionvalue(dim)
			if err != nil {
				log.Println("Fatal could not parse D", dim, err)
				return newDimensions, err
			}
			newDimensions.DDim = integerValue
		} else {
			log.Println("Fatal: unknown dimension, read dimension other than A B C or D")
		}
	}
	return newDimensions, nil
}
func givedimensionvalue(dimL string) (int, error) {
	var splittedValue = strings.Split(dimL, "=")
	if len(splittedValue) != 2 {
		log.Println("failed to read dimension value", dimL)
		return -1, errors.New("failed to read dimension value")
	}
	integerValue, err := strconv.Atoi(splittedValue[1])
	if err != nil {
		log.Println("Fatal: error converting string dimension value to integer")
	}
	return integerValue, err
}

func getsplittedIntegerValue(keyvaluepair string) (int, error) {
	var splittedpair = strings.Split(keyvaluepair, delimiter)
	if len(splittedpair) != 2 {
		return -1, errors.New("failed to split integer keyvalue pair: ")
	}
	return strconv.Atoi(splittedpair[1])
}

func getsplittedInteger64Value(keyvaluepair string) (int64, error) {
	var splittedpair = strings.Split(keyvaluepair, delimiter)
	if len(splittedpair) != 2 {
		return -1, errors.New("failed to split integer keyvalue pair")
	}
	return strconv.ParseInt(splittedpair[1], 10, 64)
}

func randomHex() string {
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatal("randomhexinsert:", err)
		return ""
	}
	return hex.EncodeToString(bytes)
}

//!!Check from client that they dont want timestamp with rawdata or is it included to raw data!
/*
	Idea is to read messages from websocket stream in nob-blocking way which means reading data in and start thread to process the data
	raw data will not be processed. it will be added as is in single string. other data will be parsed so that it can be handeled as objects
	Data  will be to add data to list until desired amount of input is added to list. When list reaches capacity it will take a lock on inserting data to list and dump data to S3.
*/


func getSecrets() {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config:            aws.Config{Region: aws.String(region_name)},
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		log.Println(err)
	}
	//keyname := "AISParameters"
	ssmsvc := ssm.New(sess, aws.NewConfig().WithRegion(region_name))
	withDecryption := false
	param, err := ssmsvc.GetParameter(&ssm.GetParameterInput{
		Name:           &parameter_name,
		WithDecryption: &withDecryption,
	})

	//param, err := ssmsvc.GetParameter(&ssm.GetParameterInput{
	//	Name:           &keyname,
	//	WithDecryption: &withDecryption,
	//})

	if err != nil {
		log.Println("Falling back to hardcoded credentials, error: ", err)
	} else {
		value := *param.Parameter.Value
		var splittedvalue = strings.Split(value, ",")
		s3bucket = splittedvalue[0]
		uusername = splittedvalue[1]
		upassword = splittedvalue[2]
		apihost = splittedvalue[3]
		apipath = splittedvalue[4]
	}
}

func main() {
	getSecrets()
	startConnect()
}

/**
Restart connection after sleep
*/

func restartConn() {
	time.Sleep(30 * time.Second)
	log.Println("Fatal error, app had to restart")
	startConnect()
}

/**
inits connection and calls restart if there is interruption


*/
func startConnect() {
	defer func() {
		restartConn()
	}()
	var queryString = "username=" + uusername + "&passwd=" + upassword
	customHeader := http.Header{}
	customHeader.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(uusername+":"+upassword)))
	flag.Parse()
	log.SetFlags(0)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	u := url.URL{Scheme: "wss", Host: apihost, Path: apipath, RawQuery: queryString}
	log.Printf("connecting to Server ", apihost, apipath)
	c, _, err := websocket.DefaultDialer.Dial(u.String(), customHeader)
	if err != nil {
		log.Fatal("dial:", err)
	}
	timeoutDuration := 1 * time.Minute

	defer c.Close()
	done := make(chan struct{})
	mux := sync.Mutex{}
	go func() {
		defer close(done)
		for {
			c.SetReadDeadline(time.Now().Add(timeoutDuration))
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Fatal("read:", err)
				time.Sleep(5 * time.Second) //wait before dumping everything and restarting
				initDumpToS3(true)          //sends parsed data to S3 bucket
				initDumpToS3(false)         // /sends raw data to S3 bucket
			}
			go messageprosessor(message, false) //sends message and informs messageinsert is not a test
			//log.Printf("recv: %s", message) //debug
		}
	}()
	mux.Lock()
	mux.Lock()
	//halts writes to lists
	parsedlock.Lock()
	lock.Lock()
	//time.Sleep(3 * time.Second) //for testing. exits after given time
	initDumpToS3(true)  //sends parsed data to S3 bucket
	initDumpToS3(false) // /sends raw data to S3 bucket
	//and unlocks for the loop
	parsedlock.Unlock()
	lock.Unlock()
}
