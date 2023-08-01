package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"ruff.io/coald/pkg/log"
)

const (
	ubusStatusEventPath = "ruff/event/status" // msg: {"csq": 33, "btn": 0}
	ubusButtonEventPath = "ruff/event/btn"    // msg: {"on": 1} or {"on": 0}
	ctrlLightOnPath     = "ruff/ctrl/lightOn" // msg: {"on": 1} or {"on": 0}
)

func lightOn(on bool) {
	var d string
	if on {
		d = `{"on":1}`
	} else {
		d = `{"on":0}`
	}
	cmd := exec.Command("ubus", "send", ctrlLightOnPath, d)

	if o, err := cmd.Output(); err != nil {
		log.Errorf("[ubus] send lightOn, data=%s , error: %v", d, err)
	} else {
		log.Infof("[ubus] sent lightOn, data=%s , got response: %s", d, o)
	}
}

func onUbusEvent(ctx context.Context, path string, outCh chan map[string]any) {
	ch := make(chan string)
	go keepListenEvent(ctx, ch, path)
	for {
		select {
		case <-ctx.Done():
			return
		case s := <-ch:
			log.Debugf("[ubus] received event, path=%q, msg=%s", path, s)
			var m map[string]any
			if err := json.Unmarshal([]byte(s), &m); err != nil {
				log.Errorf("[ubus] event %s is invalid: %v", s, err)
			} else {
				if d, ok := m[path]; !ok {
					continue
				} else if dd, ok := d.(map[string]any); ok {
					outCh <- dd
				} else {
					log.Errorf("[ubus] event %s is invalid, should be a object", s)
				}
			}
		}
	}
}

func keepListenEvent(ctx context.Context, ch chan string, path string) {
	for {
		listenEvent(ctx, ch, path)
		if ctx.Err() == nil {
			return
		}
		time.Sleep(time.Second)
	}
}

func listenEvent(ctx context.Context, ch chan string, path string) error {
	sCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
	}()

	cmd := exec.Command("ubus", "listen", path)
	p, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("[ubus] StdoutPipe : %v", err)
	}

	scanner := bufio.NewScanner(p)

	err = cmd.Start()
	go func() {
		<-sCtx.Done()
		cmd.Process.Kill()
		// if cmd.ProcessState != nil && !cmd.ProcessState.Exited() {
		// 	cmd.Process.Kill()
		// }
	}()

	if err != nil {
		return fmt.Errorf("ubus starting: %v", err)
	}

	log.Infof("[ubus] listening on %s", path)
	for scanner.Scan() {
		ch <- scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		log.Errorf("[ubus] scan: %s", err.Error())
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("[ubus] listen exit: %s", err.Error())
	}
	return nil
}
