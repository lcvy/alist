package bootstrap

import (
	"fmt"
	stdlog "log"
	"strings"
	"time"

	"github.com/alist-org/alist/v3/cmd/flags"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/db"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	_ "github.com/mutecomm/go-sqlcipher"
)

func InitDB() {
	logLevel := logger.Silent
	if flags.Debug || flags.Dev {
		logLevel = logger.Info
	}
	newLogger := logger.New(
		stdlog.New(log.StandardLogger().Out, "\r\n", stdlog.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)
	gormConfig := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: conf.Conf.Database.TablePrefix,
		},
		Logger: newLogger,
	}
	var dB *gorm.DB
	var err error
	if flags.Dev {
		dB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), gormConfig)
		conf.Conf.Database.Type = "sqlite3"
	} else {
		database := conf.Conf.Database
		switch database.Type {

			case "sqlite3":
			// 确保数据库文件名正确
			if !(strings.HasSuffix(database.DBFile, ".db") && len(database.DBFile) > 3) {
				log.Fatalf("db name error.")
			}

			// 使用 go-sqlcipher 加密连接数据库
			dsn := fmt.Sprintf("%s?_key=%s&_journal=DELETE&_vacuum=incremental", 
				database.DBFile, database.Password)

			// 打开加密的 SQLite 数据库
			dB, err = gorm.Open(sqlite.Open(dsn), gormConfig)

			// 如果数据库打开失败，尝试进行加密
			if err == nil {
				// 使用 ATTACH 语句将未加密的数据库导入到加密的数据库中
				err = dB.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS encrypted KEY '%s'", database.DBFile, database.Password)).Error
				if err != nil {
					log.Fatalf("Failed to attach database: %v", err)
				}

				// 执行加密导出
				err = dB.Exec("SELECT sqlcipher_export('encrypted');").Error
				if err != nil {
					log.Fatalf("Failed to export to encrypted database: %v", err)
				}

				// 完成后解除附加
				err = dB.Exec("DETACH DATABASE encrypted;").Error
				if err != nil {
					log.Fatalf("Failed to detach database: %v", err)
				}

				log.Info("Database successfully encrypted.")
			} else {
				log.Fatalf("Failed to open database: %v", err)
			}

	
		case "sqlite3_test":
			{
				// 处理 SQLCipher 密码
				if !(strings.HasSuffix(database.DBFile, ".db") && len(database.DBFile) > 3) {
					log.Fatalf("db name error.")
				}

				// 使用 SQLCipher 加密，修改连接字符串，设置密码
				// 注意：这里使用 _key 来传递密码
				dsn := fmt.Sprintf("%s?_key=%s&_journal=DELETE&_vacuum=incremental", 
					database.DBFile, database.Password) // 使用配置中的密码

				// 使用 SQLCipher 加密的数据库文件
				dB, err = gorm.Open(sqlite.Open(dsn), gormConfig)
			}
		case "sqlite3_1":
			{
				if !(strings.HasSuffix(database.DBFile, ".db") && len(database.DBFile) > 3) {
					log.Fatalf("db name error.")
				}
				dB, err = gorm.Open(sqlite.Open(fmt.Sprintf("%s?_journal=DELETE&_vacuum=incremental",
					database.DBFile)), gormConfig)
			}

		case "mysql":
			{
				dsn := database.DSN
				if dsn == "" {
					//[username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
					dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&tls=%s",
						database.User, database.Password, database.Host, database.Port, database.Name, database.SSLMode)
				}
				dB, err = gorm.Open(mysql.Open(dsn), gormConfig)
			}
		case "postgres":
			{
				dsn := database.DSN
				if dsn == "" {
					dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Asia/Shanghai",
						database.Host, database.User, database.Password, database.Name, database.Port, database.SSLMode)
				}
				dB, err = gorm.Open(postgres.Open(dsn), gormConfig)
			}
		default:
			log.Fatalf("not supported database type: %s", database.Type)
		}
	}
	if err != nil {
		log.Fatalf("failed to connect database:%s", err.Error())
	}
	db.Init(dB)
}
