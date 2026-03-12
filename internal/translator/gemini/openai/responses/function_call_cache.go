package responses

import "sync"

const geminiFunctionCallCacheLimit = 64

type geminiFunctionCallCache struct {
	mu        sync.Mutex
	order     []string
	responses map[string]map[string]string
}

var cachedGeminiFunctionCalls = &geminiFunctionCallCache{
	responses: make(map[string]map[string]string),
}

func rememberGeminiFunctionCalls(responseID string, names map[string]string) {
	if responseID == "" || len(names) == 0 {
		return
	}

	cachedGeminiFunctionCalls.mu.Lock()
	defer cachedGeminiFunctionCalls.mu.Unlock()

	copied := make(map[string]string, len(names))
	for callID, name := range names {
		if callID == "" || name == "" {
			continue
		}
		copied[callID] = name
	}
	if len(copied) == 0 {
		return
	}

	if _, exists := cachedGeminiFunctionCalls.responses[responseID]; !exists {
		cachedGeminiFunctionCalls.order = append(cachedGeminiFunctionCalls.order, responseID)
	}
	cachedGeminiFunctionCalls.responses[responseID] = copied

	for len(cachedGeminiFunctionCalls.order) > geminiFunctionCallCacheLimit {
		evicted := cachedGeminiFunctionCalls.order[0]
		cachedGeminiFunctionCalls.order = cachedGeminiFunctionCalls.order[1:]
		delete(cachedGeminiFunctionCalls.responses, evicted)
	}
}

func lookupGeminiFunctionCallName(responseID, callID string) string {
	if responseID == "" || callID == "" {
		return ""
	}

	cachedGeminiFunctionCalls.mu.Lock()
	defer cachedGeminiFunctionCalls.mu.Unlock()

	if calls := cachedGeminiFunctionCalls.responses[responseID]; calls != nil {
		return calls[callID]
	}
	return ""
}
