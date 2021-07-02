package config

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/viper"
)

var ProConf = new(Config)

type Config struct {
	OPQ   OPQConfig
	MySQL MySQLConfig
}

type OPQConfig struct {
	Url string
	QQ  int64
}

// MySQLConfig 关系数据库配置
type MySQLConfig struct {
	Host string
	Port int
	User string
	PWD  string
	DB   string
}

// 主地址配置
var (
	DBMain string
)

func init() {

	// 取项目地址
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	viper.AddConfigPath(path)     // 设置读取的文件路径
	viper.SetConfigName("config") // 设置读取的文件名
	viper.SetConfigType("yaml")   // 设置文件的类型

	err = viper.ReadInConfig() // 读取配置信息
	if err != nil {            // 读取配置信息失败
		log.Fatalln(err)
	}
	// 将读取的配置信息保存至全局变量Conf
	if err := viper.Unmarshal(ProConf); err != nil {
		log.Fatalln(err)
	}

	// 拼接最终数据库连接地址
	DBMain = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", ProConf.MySQL.User, ProConf.MySQL.PWD, ProConf.MySQL.Host, ProConf.MySQL.Port, ProConf.MySQL.DB)

}
