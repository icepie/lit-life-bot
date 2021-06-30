package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

var ProConf = new(Config)

type Config struct {
	OPQ OPQConfig
}

type OPQConfig struct {
	Url string
	QQ  int64
}

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

}
