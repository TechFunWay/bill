package version

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
	AppName   = "bill"
)

func GetVersion() map[string]string {
	return map[string]string{
		"version":   Version,
		"buildTime": BuildTime,
		"gitCommit": GitCommit,
		"appName":   AppName,
	}
}

func PrintVersion() {
	println(AppName, Version)
	println("Version:", Version)
	println("BuildTime:", BuildTime)
	println("GitCommit:", GitCommit)
}
