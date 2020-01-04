package logger

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	LogDir         string
	LogFileMaxSize int64
	TimeLocation   *time.Location
	ChannelSize    int
	OutputScreen   bool
}

const (
	debugLevel = "dbg"
	infoLevel  = "inf"
	warnLevel  = "wrn"
	errorLevel = "err"

	fileName    = "2006-01-02"
	timeFormart = "2006-01-02 15:04:05"
)

var (
	logDir               = "./log/"
	timeLocation         = time.Now().Location()
	logFileMaxSize int64 = 1 * 1024 * 1024 * 1024
	outputScreen         = false

	date    string
	started bool
	fileMap map[string]*os.File
	logChan chan *Log
)

func Init(config Config) {
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
	date = time.Now().In(timeLocation).Format(fileName)
	fileDir := logDir + "/" + level + "/"
	var fileName = path.Clean(fileDir + date + ".log")
	ok := pathExists(fileDir)
	if !ok {
		if err := os.MkdirAll(fileDir, os.ModePerm); err != nil {
			panic(err)
		}
	} else {
		if getFileSize(fileName) > logFileMaxSize {
			renameFile(fileDir)
		}
	}
	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0664)
	if err != nil {
		return nil
	}
	return file
}

func renameFile(infoFileDir string) {
	files, _ := ioutil.ReadDir(infoFileDir)
	var todayFile []os.FileInfo
	for _, onefile := range files {
		fileName := onefile.Name()
		if !onefile.IsDir() && strings.Index(fileName, date) == 0 {
			todayFile = append(todayFile, onefile)
		}
	}
	for i := 0; i < len(todayFile)-1; i++ {
		for j := i + 1; j < len(todayFile); j++ {
			if getSuffix(todayFile[i].Name()) < getSuffix(todayFile[j].Name()) {
				todayFile[i], todayFile[j] = todayFile[j], todayFile[i]
			}
		}
	}
	for i := range todayFile {
		os.Rename(infoFileDir+todayFile[i].Name(),
			infoFileDir+fmt.Sprintf("%s_%d.log", date, len(todayFile)-i))
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
	if file, ok = fileMap[log.Level]; ok {
		stat, _ := file.Stat()
		if stat.Size() > logFileMaxSize ||
			strings.Index(log.Time.In(timeLocation).
				Format(fileName), date) != 0 {
			needNewFile = true
		}
	} else {
		needNewFile = true
	}

	if needNewFile {
		if file != nil {
			file.Close()
		}
		file = newFile(log.Level)
		fileMap[log.Level] = file
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
		result = result + fmt.Sprintf("%s [%s] [%s] %s",
			log.Time.In(timeLocation).Format(timeFormart),
			strings.ToUpper(log.Level),
			log.Line,
			msgList[i]) + "\n"
	}
	if outputScreen {
		fmt.Print(result)
	}
	return result
}
