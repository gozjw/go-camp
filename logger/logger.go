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
}

const (
	DebugLevel = "dbg"
	InfoLevel  = "inf"
	WarnLevel  = "wrn"
	ErrorLevel = "err"

	fileName    = "2006-01-02"
	timeFormart = "2006-01-02 15:04:05"
)

var (
	logDir               = "./log/"
	timeLocation         = time.Now().Location()
	logFileMaxSize int64 = 1 * 1024 * 1024 * 1024

	date    string
	started bool

	infoFile *os.File

	infoChan chan *LogInfo
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

	if config.ChannelSize == 0 {
		infoChan = make(chan *LogInfo, 1024)
	} else {
		infoChan = make(chan *LogInfo, config.ChannelSize)
	}

	go writeLog()
}

type LogInfo struct {
	Level   string
	Time    time.Time
	Line    string
	Message string
}

func Push(data LogInfo) {
	if !started {
		panic("logger not start")
	}
	infoChan <- &data
}

func Debug(params ...interface{}) {
	info(DebugLevel, params...)
}

func Info(params ...interface{}) {
	info(InfoLevel, params...)
}

func Warn(params ...interface{}) {
	info(WarnLevel, params...)
}

func Error(params ...interface{}) {
	info(ErrorLevel, params...)
}

func Debugf(format string, params ...interface{}) {
	info(DebugLevel, fmt.Sprintf(format, params...))
}

func Infof(format string, params ...interface{}) {
	info(InfoLevel, fmt.Sprintf(format, params...))
}

func Warnf(format string, params ...interface{}) {
	info(WarnLevel, fmt.Sprintf(format, params...))
}

func Errorf(format string, params ...interface{}) {
	info(ErrorLevel, fmt.Sprintf(format, params...))
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
	info := LogInfo{
		Level:   level,
		Time:    time.Now(),
		Line:    fmt.Sprintf("%s:%d", file, line),
		Message: message,
	}
	Push(info)
}

func newInfoFile() {
	if infoFile != nil {
		infoFile.Close()
	}
	infoFile = newFile(InfoLevel)
}

func newFile(level string) *os.File {
	date := time.Now().In(timeLocation).Format(fileName)
	fileDir := logDir + "/" + level + "/"
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

func renameFile(infoFileDir, dayDate string) {
	files, _ := ioutil.ReadDir(infoFileDir)
	var todayFile []os.FileInfo
	dayDate = time.Now().Format(fileName)
	for _, onefile := range files {
		fileName := onefile.Name()
		if !onefile.IsDir() && strings.Index(fileName, dayDate) == 0 {
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
			infoFileDir+fmt.Sprintf("%s_%d.log", dayDate, len(todayFile)-i))
	}
}

func writeLog() {
	started = true
	defer func() { started = false }()
	for {
		select {
		case info := <-infoChan:
			if info.Level == DebugLevel || info.Level == WarnLevel || info.Level == ErrorLevel {
				file := newFile(info.Level)
				file.WriteString(formatLine(info))
				file.Close()
			} else {
				if infoFile == nil {
					newInfoFile()
				}
				stat, _ := infoFile.Stat()
				if stat.Size() > logFileMaxSize ||
					strings.Index(info.Time.In(timeLocation).Format(fileName), date) != 0 {
					newInfoFile()
				}
				infoFile.WriteString(formatLine(info))
			}
		}
	}
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

func formatLine(info *LogInfo) string {
	var result = ""
	msgList := strings.Split(info.Message, "\n")
	for i := range msgList {
		result = result + fmt.Sprintf("%s [%s] [%s] %s",
			info.Time.In(timeLocation).Format(timeFormart),
			strings.ToUpper(info.Level),
			info.Line,
			msgList[i]) + "\n"
	}
	fmt.Print(result)
	return result
}
