# Koolo Bot Project - Comprehensive Analysis Report

## Summary
The Koolo bot project is a substantial Go application (300 Go files) for automating Diablo 2 gameplay. While functional, it has several categories of issues affecting code quality, reliability, security, and maintainability.

---

## 1. CODE QUALITY ISSUES

### 1.1 Unused Variables and Dead Code
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/cmd/koolo/main.go`
- **Lines 76-77:** Explicitly ignoring buildID and buildTime variables
  ```go
  _ = buildID
  _ = buildTime
  ```
- **Impact:** These variables are set by build process but never used; indicates incomplete implementation

### 1.2 Missing Error Handling
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/bot/scheduler.go`
- **Line 744:** Errors ignored in json.MarshalIndent
  ```go
  data, _ = json.MarshalIndent(history, "", "  ")
  ```
- **Line 745:** os.WriteFile errors not handled
  ```go
  os.WriteFile(path, data, 0644)  // No error check
  ```
- **Impact:** Silent failures when saving scheduler state; data loss possible

**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/bot/bot.go`
- **Line 223:** Bare recover() without error handling
  ```go
  defer func() {
      cancel()
      b.Stop()
      recover()  // Does nothing - panic still propagates!
  }()
  ```
- **Impact:** Panics are not actually recovered; goroutine terminates unexpectedly

### 1.3 Improper Panic Usage for Control Flow
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/chicken/chicken.go`
- **Lines 19, 23, 27, 31:** Using panic for conditional logic
  ```go
  panic(fmt.Errorf("%w: Player has amplify damage curse", health.ErrChicken))
  ```
- **File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/context/context.go`
- **Line 229:** Panic used for stopping bot
  ```go
  panic("Bot is stopped")
  ```
- **Impact:** Anti-pattern; should use errors or explicit stop mechanism; makes debugging difficult

### 1.4 Bare recover() Calls
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/bot/bot.go`
- **Line 223:** `recover()` without assigning to variable
- **Impact:** Silently suppresses panic without handling; masks errors

### 1.5 Debugging Code Left in Production
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/action/stash.go`
- **Lines 280, 282, 287, 296, 330, 334, 343:** Multiple fmt.Printf debug statements
  ```go
  fmt.Printf("DEBUG: Evaluating item '%s' for *absolute* exclusion from stash.\n", i.Name)
  ```
- **File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/action/Inventory.go`
- **Line 319:** Commented debug code
  ```go
  //fmt.Printf("Optimization complete. Best free rectangle: %dx%d (score %.2f)\n", w, h, s)
  ```
- **File:** Multiple files with fmt.Println/Printf
- **Impact:** Bypasses logging system; output not captured in logs; clutters output

### 1.6 Global State Management
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/cmd/koolo/log/logger.go`
- **Line 15:** Global `logFileHandler` variable
  ```go
  var logFileHandler *os.File
  ```
- **Lines 17-22, 34:** No synchronization for concurrent access
  ```go
  func FlushLog() {
      if logFileHandler != nil {
          logFileHandler.Sync()  // Race condition!
      }
  }
  ```
- **Impact:** Thread-unsafe; multiple goroutines calling Flush/Close can race

**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/context/context.go`
- **Lines 21-22:** Global maps without synchronization
  ```go
  var mu sync.Mutex
  var botContexts = make(map[uint64]*Status)
  ```
- **Line 154:** Heavy operation called frequently
  ```go
  func Get() *Status {
      mu.Lock()
      defer mu.Unlock()
      return botContexts[getGoroutineID()]  // Calls runtime.Stack() every time!
  }
  ```
- **Impact:** Performance issue; runtime.Stack() is expensive when called 430+ times per run

### 1.7 String Concatenation for Paths
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/cmd/koolo/log/logger.go`
- **Line 36:** Using string concatenation instead of filepath.Join
  ```go
  lfh, err := os.Create(logDir + "/" + fileName)
  ```
- **Impact:** Not portable; doesn't handle Windows paths correctly; directory separator issues

---

## 2. GOROUTINE AND CONCURRENCY ISSUES

### 2.1 Goroutine Leaks
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/cmd/koolo/main.go`
- **Line 128:** Scheduler started without context cancellation
  ```go
  go scheduler.Start()
  ```
- **Line 150:** Auto-save polling goroutine without context propagation
  ```go
  go func() {
      // ... no ctx.Done() check in select
  }()
  ```
- **Line 177:** Window title rotation goroutine
  ```go
  go func() {
      for {
          // Runs indefinitely; relies on context
      }
  }()
  ```
- **Impact:** If context is never cancelled properly, goroutines may leak

### 2.2 Unsafe Pointer Usage
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/cmd/koolo/main.go`
- **Line 218:** Unsafe pointer cast for RECT struct
  ```go
  ret, _, _ := getWindowRect.Call(handle, uintptr(unsafe.Pointer(&rect)))
  ```
- **Line 242:** Unsafe pointer cast for window title
  ```go
  winproc.SetWindowText.Call(handle, uintptr(unsafe.Pointer(titlePtr)))
  ```
- **Impact:** While likely correct, unsafe code requires careful verification; GC could move stack

### 2.3 Potential Goroutine Leak in Manager
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/bot/manager.go`
- **Line 99-102:** Panic recovery in Start() but error not propagated
  ```go
  defer func() {
      if r := recover(); r != nil {
          mng.logger.Error("PANIC in manager.Start", ...)
      }
  }()
  ```
- **Impact:** Supervisor goroutine starts but startup failure silently logged

---

## 3. SECURITY ISSUES

### 3.1 CORS Vulnerability - Unrestricted WebSocket Origins
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/server/http_server.go`
- **Lines 84-88:** All origins accepted for WebSocket
  ```go
  upgrader = websocket.Upgrader{
      CheckOrigin: func(r *http.Request) bool {
          return true  // CRITICAL: Accepts all origins
      },
  }
  ```
- **Severity:** HIGH
- **Impact:** Cross-site WebSocket hijacking (CSWSH); remote webpage can connect to bot's WebSocket
- **Recommendation:** Validate Origin header against whitelist:
  ```go
  CheckOrigin: func(r *http.Request) bool {
      origin := r.Header.Get("Origin")
      return origin == "http://localhost:8087"
  }
  ```

### 3.2 Hardcoded Paths in Shell Script
**File:** `/home/jnrabit/projekte/koolo bot/start_koolo_rotw.sh`
- **Lines 7-10:** Absolute paths hardcoded
  ```bash
  KOOLO_DIR="/home/jnrabit/.var/app/com.usebottles.bottles/data/bottles/bottles/Gaming/drive_c/koolo-rotw"
  STEAM_DIR="/home/jnrabit/.local/share/Steam"
  ```
- **Impact:** Not portable; breaks if user's home directory changes or setup varies

### 3.3 Debug Port Exposed
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/cmd/koolo/main.go`
- **Line 9:** pprof import for side effects
  ```go
  _ "net/http/pprof"
  ```
- **Impact:** If HTTP server starts, debug endpoints are exposed on all interfaces
- **Current status:** No evidence of serving pprof endpoints, but import suggests it could be enabled

### 3.4 No Authentication on Local Server
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/server/http_server.go`
- **Observation:** No authentication mechanism on API endpoints
- **Impact:** Any process on localhost can control bot; LAN exposure if firewall misconfigured

---

## 4. RESOURCE MANAGEMENT ISSUES

### 4.1 File Handle Not Closed on Error
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/cmd/koolo/log/logger.go`
- **Line 37-40:** File handle assigned to global without defer
  ```go
  lfh, err := os.Create(logDir + "/" + fileName)
  if err != nil {
      return nil, err  // If error returned later, lfh never closed
  }
  logFileHandler = lfh
  ```
- **Impact:** Potential file descriptor leak if error occurs after Create

### 4.2 Missing Resource Cleanup in Error Paths
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/cmd/koolo/main.go`
- **Line 140:** ngrokTunnel could leak if error occurs after creation
- **Impact:** Tunnel resources not released on error

---

## 5. ARCHITECTURE AND DESIGN ISSUES

### 5.1 Missing Abstractions - Tight Coupling
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/context/context.go`
- **Line 21-22:** Global context map tightly couples all packages to context mechanism
- **Usage:** 430+ calls to context.Get() throughout codebase
- **Impact:** Makes testing difficult; hard to inject dependencies; no clear data flow

### 5.2 Dependency on External Local Module
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/go.mod`
- **Line 50:** Local path replacement
  ```
  replace github.com/hectorgimenez/d2go => ../d2go-local
  ```
- **Impact:** Not reproducible build; requires d2go-local in parent directory; breaks CI/CD

### 5.3 No Dependency Injection
- **Impact:** Hard to mock dependencies for testing
- **Seen in:** manager.go, bot.go, scheduler.go - all take logger/manager/scheduler as constructor params but use global context.Get()

### 5.4 Poor Separation of Concerns
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/server/http_server.go`
- **Size:** 4700+ lines
- **Responsibilities:** HTTP routing, WebSocket management, PickIT API, Drop API, Runewood API, template rendering, UI data preparation
- **Impact:** Single file handles too many concerns; hard to test/maintain

### 5.5 Service Locator Anti-pattern
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/context/context.go`
- **Line 154-159:** Uses runtime.Stack() to find caller's goroutine ID
  ```go
  func Get() *Status {
      mu.Lock()
      defer mu.Unlock()
      return botContexts[getGoroutineID()]  // Service locator via goroutine ID
  }
  ```
- **Impact:** Obscures dependencies; makes code flow hard to follow; expensive operation

---

## 6. TESTING GAPS

### 6.1 Minimal Test Coverage
- **Total Go files:** 300
- **Test files:** 1 (astar_test.go)
- **Test coverage:** ~0.3%
- **Impact:** High risk of regressions; hard to refactor safely

### 6.2 No Unit Tests for Critical Components
- No tests for: manager.go, bot.go, scheduler.go, config parsing, error handling
- **Impact:** Bugs in core business logic go undetected

### 6.3 No Integration Tests
- **Impact:** Interaction between components untested

---

## 7. CONFIGURATION AND DEPLOYMENT ISSUES

### 7.1 Shell Script Issues
**File:** `/home/jnrabit/projekte/koolo bot/start_koolo_rotw.sh`

#### 7.1.1 Hardcoded Configuration
- **Lines 7-10, 17-18:** User name, paths, CPU cores, priority levels hardcoded
- **Recommendation:** Use environment variables with defaults:
  ```bash
  KOOLO_DIR="${KOOLO_DIR:-/home/jnrabit/...}"
  TASKSET_CPUS="${TASKSET_CPUS:-6-7,14-15}"
  ```

#### 7.1.2 No Error Handling on Kill
- **Line 19:** `pkill` errors ignored
  ```bash
  pkill -f "proton.*koolo\.exe" 2>/dev/null
  ```
- **Recommendation:** Check if already running before attempting restart

#### 7.1.3 Hardcoded Sleep Duration
- **Line 20:** Fixed 2-second sleep may not be enough for cleanup
  ```bash
  sleep 2
  ```
- **Recommendation:** Use configurable timeout or check process state

#### 7.1.4 Unverified PROTON Path
- **Line 10:** Assumes PROTON executable exists; no validation
- **Recommendation:** Check executable before use:
  ```bash
  [ -x "$PROTON" ] || { echo "Proton not found"; exit 1; }
  ```

#### 7.1.5 No Graceful Shutdown Handler
- **Impact:** Ctrl+C in script doesn't gracefully stop bot
- **Recommendation:** Add trap handler:
  ```bash
  trap "kill %1; wait" EXIT
  nohup ... & wait
  ```

#### 7.1.6 Log File Not Rotated
- **Line 32:** Fixed log file path /tmp/koolo-rotw.log
- **Impact:** Unlimited growth; gets wiped on reboot
- **Recommendation:** Implement log rotation

### 7.2 Go Module Issues
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/go.mod`
- **Line 50:** Local replacement not suitable for production
  ```
  replace github.com/hectorgimenez/d2go => ../d2go-local
  ```
- **Impact:** Build breaks if d2go-local not in parent directory
- **Recommendation:** Use versioned dependency from registry

### 7.3 No Configuration Validation at Startup
- **Impact:** Invalid configs are only discovered at runtime
- **Seen in:** main.go - loads config but doesn't validate completeness

---

## 8. ERROR HANDLING PATTERNS

### 8.1 Inconsistent Error Handling
- **Pattern 1 (Good):** config.go line 1093
  ```go
  err = os.WriteFile("config/koolo.yaml", text, 0644)
  if err != nil {
      return fmt.Errorf("error writing koolo config: %w", err)
  }
  ```
- **Pattern 2 (Bad):** scheduler.go line 745
  ```go
  os.WriteFile(path, data, 0644)  // Ignored
  ```
- **Pattern 3 (Silent):** scheduler.go line 744
  ```go
  data, _ = json.MarshalIndent(history, "", "  ")
  ```
- **Impact:** Some errors logged, some ignored, some silently suppressed

### 8.2 Panic Not Recovered Properly
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/bot/bot.go`
- **Line 223:** Bare recover()
- **Lines 401-405:** Proper panic conversion to error
  ```go
  if r := recover(); r != nil {
      if e, ok := r.(error); ok && errors.Is(e, health.ErrChicken) {
          returnErr = e
      }
  }
  ```
- **Impact:** Inconsistent panic handling across codebase

---

## 9. LOGGING ISSUES

### 9.1 Bypassing Logging System
**Locations:**
- `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/action/stash.go` - 8 fmt.Printf statements
- `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/character/sorceress_leveling.go` - 3 fmt.Printf statements  
- `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/game/manager.go:427` - GPU initialization
- `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/game/packet_sender.go:57` - Teleport packet debug
- `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/run/fireeye.go:44` - fmt.Println
- `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/run/tristramearlygoldfarm.go` - Multiple fmt.Println

**Impact:** Debug output not captured in logs; output lost when redirected; inconsistent logging

### 9.2 No Request Logging Middleware
**File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/server/http_server.go`
- No HTTP request logging for API calls
- **Impact:** Hard to debug API issues; no audit trail

---

## 10. ADDITIONAL CONCERNS

### 10.1 Memory Usage Not Optimized
- **File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/context/context.go`
- **Issue:** runtime.Stack() called on every context.Get() (430+ times per run)
- **Impact:** Each call allocates and parses stack trace; significant CPU/memory overhead

### 10.2 No Rate Limiting
- **File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/internal/server/http_server.go`
- **Impact:** API endpoints could be abused; no protection against DoS

### 10.3 Window Title Rotation Entropy
- **File:** `/home/jnrabit/projekte/koolo bot/koolo-rotw/cmd/koolo/main.go`
- **Lines 237-252:** Uses math/rand without seeding
- **Impact:** Predictable pseudo-random sequence; may not be effective for obfuscation

### 10.4 Temporal Coupling in Shell Script
- **Line 20:** `sleep 2` assumes cleanup time
- **Impact:** Race condition if cleanup takes longer

---

## PRIORITIZED RECOMMENDATIONS

### Critical (Fix Immediately)
1. **CORS Vulnerability** - Allow only localhost origins for WebSocket
2. **Error Handling in Scheduler** - Handle os.WriteFile and json.Marshal errors
3. **Bare recover()** - Properly convert panics to errors or log/handle them
4. **File Handle Leak** - Use defer for file operations

### High (Fix Soon)
5. **Panic for Control Flow** - Replace panic() with proper error returns
6. **Global State Synchronization** - Add sync.Mutex to logFileHandler access
7. **Goroutine Leaks** - Ensure all goroutines respect context cancellation
8. **Testing** - Add unit tests for critical components
9. **Debug Code Cleanup** - Remove or gate debug fmt.Printf statements
10. **Local Module Replacement** - Use versioned dependency for d2go

### Medium (Improve Quality)
11. **Logger Usage** - Replace fmt.Printf with logger calls
12. **Path Handling** - Use filepath.Join instead of string concatenation
13. **Refactor Large File** - Split http_server.go into multiple files
14. **Add Middleware** - Implement HTTP request logging
15. **Performance** - Cache goroutine ID or use explicit context parameter instead of context.Get()

### Low (Technical Debt)
16. **Configuration Validation** - Add startup validation
17. **Shell Script Hardening** - Use env vars, check executables
18. **Log Rotation** - Implement rotating file handler
19. **Rate Limiting** - Add API rate limiting
20. **Dead Code** - Remove buildID/buildTime if unused

