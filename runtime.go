package noscronjob

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

// getHandlerName extracts the handler function name by reflection.
//
//	func (c *CronHandlers) MyGoCronJob(ctx *noscronjob.Context) { <=== Extract name "MyGoCronJob"
//		//....
//	}
func getHandlerName(handler HandlerFunc) (string, bool) {
	ptr := reflect.ValueOf(handler).Pointer()
	handlerName := runtime.FuncForPC(ptr).Name()

	// output: "cronjob.ChatroomCronjobs.BroadcastChatroomMemberCountV2-fm"
	base := filepath.Base(handlerName)

	// output: ["cronjob", "ChatroomCronjobs", "BroadcastChatroomMemberCountV2-fm"]
	namespaces := strings.Split(base, ".")

	if len(namespaces) == 0 {
		return "", false
	}

	// output: "BroadcastChatroomMemberCountV2-fm"
	element := namespaces[len(namespaces)-1]

	// output: []string{"BroadcastChatroomMemberCountV2", "m"}
	splitted := strings.Split(element, "-")

	if len(splitted) == 0 {
		return "", false
	}

	return splitted[0], true
}
