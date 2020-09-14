package logger

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	LogDir         string
	LogFileMaxSize int64
	TimeLocation   *time.Location
	ChannelSize    int
	OutputScreen   bool
	UseColor       bool
}

const (
	debugLevel = "DBG"
	infoLevel  = "INF"
	warnLevel  = "WRN"
	errorLevel = "ERR"

	fileName    = "2006-01-02"
	timeFormart = "2006-01-02 15:04:05"
)

var (
	logDir               = "./log/"
	timeLocation         = time.Now().Location()
	logFileMaxSize int64 = 1 * 1024 * 1024 * 1024
	logChan              = make(chan *Log, 1024)

	outputScreen bool
	useColor     bool
	started      bool
	fileMap      map[string]*os.File
	mux          sync.Mutex
)

func Init(config Config) {
	if started {
		panic("logger started")
	}
	if config.LogDir != "" {
		logDir = config.LogDir
	}
	if config.LogFileMaxSize != 0 {
		logFileMaxSize = config.LogFileMaxSize
	}
	if config.TimeLocation != nil {
		timeLocation = config.TimeLocation
	}
	outputScreen = config.OutputScreen
	useColor = config.UseColor
	if config.ChannelSize == 0 {
		logChan = make(chan *Log, 1024)
	} else {
		logChan = make(chan *Log, config.ChannelSize)
	}
	fileMap = make(map[string]*os.File)

	go start()
}

type Log struct {
	Level   string
	Time    time.Time
	Line    string
	Message string
}

func Push(log Log) {
	mux.Lock()
	defer mux.Unlock()
	if !started {
		panic("logger not start")
	}
	logChan <- &log
}

func Debug(params ...interface{}) {
	info(debugLevel, params...)
}

func Info(params ...interface{}) {
	info(infoLevel, params...)
}

func Warn(params ...interface{}) {
	info(warnLevel, params...)
}

func Error(params ...interface{}) {
	info(errorLevel, params...)
}

func Debugf(format string, params ...interface{}) {
	info(debugLevel, fmt.Sprintf(format, params...))
}

func Infof(format string, params ...interface{}) {
	info(infoLevel, fmt.Sprintf(format, params...))
}

func Warnf(format string, params ...interface{}) {
	info(warnLevel, fmt.Sprintf(format, params...))
}

func Errorf(format string, params ...interface{}) {
	info(errorLevel, fmt.Sprintf(format, params...))
}

func info(level string, params ...interface{}) {
	var message string
	var messageList []string
	for _, p := range params {
		messageList = append(messageList, fmt.Sprintf("%+v", p))
	}
	message = strings.Join(messageList, " ")
	function, _, _, _ := runtime.Caller(2)
	file, line := runtime.FuncForPC(function).FileLine(function)
	log := Log{
		Level:   level,
		Time:    time.Now(),
		Line:    fmt.Sprintf("%s:%d", file, line),
		Message: message,
	}
	Push(log)
}

func newFile(level string) *os.File {
	date := time.Now().In(timeLocation).Format(fileName)
	fileDir := logDir + "/" + strings.ToLower(level) + "/"
	var fileName = path.Clean(fileDir + date + ".log")
	ok := pathExists(fileDir)
	if !ok {
		if err := os.MkdirAll(fileDir, os.ModePerm); err != nil {
			panic(err)
		}
	} else {
		if getFileSize(fileName) > logFileMaxSize {
			renameFile(fileDir, date)
		}
	}
	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0664)
	if err != nil {
		return nil
	}
	return file
}

func renameFile(fileDir, date string) {
	files, _ := ioutil.ReadDir(fileDir)
	var dateFile []os.FileInfo
	for _, onefile := range files {
		fileName := onefile.Name()
		if !onefile.IsDir() && strings.Index(fileName, date) == 0 {
			dateFile = append(dateFile, onefile)
		}
	}
	for i := 0; i < len(dateFile)-1; i++ {
		for j := i + 1; j < len(dateFile); j++ {
			if getSuffix(dateFile[i].Name()) < getSuffix(dateFile[j].Name()) {
				dateFile[i], dateFile[j] = dateFile[j], dateFile[i]
			}
		}
	}
	for i := range dateFile {
		os.Rename(fileDir+dateFile[i].Name(),
			fileDir+fmt.Sprintf("%s_%d.log", date, len(dateFile)-i))
	}
}

func start() {
	started = true
	defer func() { started = false }()
	for {
		select {
		case log := <-logChan:
			write(log)
		}
	}
}

func write(log *Log) {
	var file *os.File
	var ok bool
	var needNewFile bool
	var level string

	switch log.Level {
	case debugLevel, warnLevel, errorLevel:
		level = log.Level
	default:
		level = infoLevel
	}

	if file, ok = fileMap[level]; ok {
		stat, _ := file.Stat()
		date := strings.Split(stat.Name(), ".")[0]
		if stat.Size() > logFileMaxSize ||
			strings.Index(log.Time.In(timeLocation).Format(fileName), date) != 0 {
			needNewFile = true
		}
	} else {
		needNewFile = true
	}

	if needNewFile {
		if file != nil {
			file.Close()
		}
		file = newFile(level)
		fileMap[level] = file
	}

	file.WriteString(formatLine(log))
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func getFileSize(path string) int64 {
	if !pathExists(path) {
		return 0
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}

func getSuffix(fileName string) int {
	suffix := 0
	srts := strings.Split(fileName, "_")
	if len(srts) > 1 {
		srts := strings.Split(srts[1], ".")
		suffix, _ = strconv.Atoi(srts[0])
	}
	return suffix
}

func formatLine(log *Log) string {
	var result = ""
	msgList := strings.Split(log.Message, "\n")
	for i := range msgList {
		if useColor {
			log.Level = setLevelColor(log.Level)
		}
		result = result + fmt.Sprintf("%s [%s] [%s] %s",
			log.Time.In(timeLocation).Format(timeFormart),
			log.Level,
			log.Line,
			msgList[i]) + "\n"
	}

	if outputScreen {
		fmt.Print(result)
	}

	return result
}

func setLevelColor(level string) string {
	var color int
	switch level {
	case infoLevel:
		color = 32
	case debugLevel:
		color = 34
	case warnLevel:
		color = 33
	case errorLevel:
		color = 31
	default:
		color = 36
	}
	return fmt.Sprintf("\033[%dm%s\033[0m", color, level)
}
