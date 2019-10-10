package setting

import (
	"flag"
	"fmt"
	"os"

	"github.com/tossp/tsgo/pkg/utils"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	defAcm = `[request_definition]
r = sub, obj, act, service

[policy_definition]
p = sub, obj, act, service, eft

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = (r.service == p.service || p.service=="*") && ( g(r.sub, p.sub) || p.sub=="*") && (keyMatch2(r.obj, p.obj) || keyMatch(r.obj, p.obj)) && regexMatch(r.act, p.act)`
)

func config() {
	defConfig()

	viper.SetEnvPrefix("ts")
	viper.AutomaticEnv()
	_ = viper.BindEnv(DataDirKey)
	_ = viper.BindEnv(ConfigDirKey)

	flag.String(ConfigDirKey, viper.GetString(ConfigDirKey), "基础配置文件路径")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	_ = viper.BindPFlags(pflag.CommandLine)

	_ = os.MkdirAll(viper.GetString(ConfigDirKey), 0755)

	viper.SetConfigName(configFileName)
	viper.AddConfigPath(viper.GetString(ConfigDirKey))
	viper.AddConfigPath(UseAppPath("configs"))
	viper.AddConfigPath(appPath)
	viper.AddConfigPath("$HOME/.ts_site")
	viper.AddConfigPath(".")
	configFN := joinPath(viper.GetString(ConfigDirKey), configFileName+".toml")
read:
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("配置文件错误: %s\n", err)
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			f, fuck := os.OpenFile(configFN, os.O_RDONLY|os.O_CREATE, 0644)
			if fuck != nil {
				fmt.Printf("尝试修复配置文件错误: %s\n", fuck)
				os.Exit(1)
			}
			_ = f.Close()
			fmt.Printf("修复配置: %s\n", configFN)
			goto read
		}
		panic(err)
	}
	overwrite()
	fmt.Printf("加载配置文件: %s\n", viper.ConfigFileUsed())
	write()
}
func overwrite() {
	viper.Set("accessControl.model", defAcm)
	_ = os.MkdirAll(viper.GetString(DataDirKey), 0755)
	_ = os.MkdirAll(viper.GetString(ConfigDirKey), 0755)
}

func watch() {
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("配置发生变更：", e.Op.String())
		oldConfig := viper.New()
		for _, v := range viper.AllKeys() {
			oldConfig.Set(v, viper.Get(v))
		}
		if err := viper.ReadInConfig(); err != nil {
			fmt.Printf("重新加载配置文件错误: %s\n", viper.ConfigFileUsed())
			for _, v := range oldConfig.AllKeys() {
				viper.Set(v, viper.Get(v))
			}
			write()
			return
		}
		overwrite()
		fmt.Printf("重新加载配置文件: %s\n", viper.ConfigFileUsed())
	})
}

func write() {
	err := viper.WriteConfig()
	if err != nil {
		fmt.Printf("保存配置文件错误: %s\n", err)
	}
}

func defConfig() {
	viper.SetDefault(DataDirKey, UseAppPath("data"))
	viper.SetDefault(ConfigDirKey, UseAppPath("configs"))
	viper.SetDefault("secret", utils.GetRandomString(32))
	viper.SetDefault("db.User", "ts")
	viper.SetDefault("db.Password", "123456")
	viper.SetDefault("db.Prefix", "ts")
	viper.SetDefault("db.Host", "127.0.0.1")
	viper.SetDefault("db.Port", 5432)
	viper.SetDefault("db.Name", "ts")
	viper.SetDefault("db.Ssl_mode", "disable")
	viper.SetDefault("db.Max_Idle_Conns", 10)
	viper.SetDefault("db.Max_Open_Conns", 20)
	viper.SetDefault("web.address", ":2080")
	viper.SetDefault("accessControl.Enable", true)
	viper.SetDefault("accessControl.Model", defAcm)
	viper.SetDefault("storage.Bucket", "sites")
	viper.SetDefault("storage.Endpoint", "127.0.0.1")
	viper.SetDefault("storage.AccessKey", "Q3AM3UQ867SPQQA43P2F")
	viper.SetDefault("storage.SecretKey", "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG")
	viper.SetDefault("storage.Secure", true)
}
