package keys

func Resolve(key string) (Action, bool) {
	action, ok := defaults[key]
	return action, ok
}
