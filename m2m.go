package main

import (
	"context"
	"encoding/json"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"ruff.io/coald/pkg/log"
)

const (
	m2mTopicTmpl = "$iothub/custom/things/{thingId}/m2m/src"
)

type M2MMsg struct {
	Typ  string `json:"type"` // lightOn
	Data any    `json:"data"` // 1 / 0 for lightOn
}

func m2mCtlLoop(ctx context.Context, thingId string, client mqtt.Client) {
	topic := m2mTopic(thingId)
	ch := make(chan map[string]any)
	go onUbusEvent(ctx, ubusButtonEventPath, ch)
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-ch:
			log.Debugf("[m2m] received event: %v", e)
			if on, ok := e["on"]; ok {
				t := false
				if v, ok := on.(float64); ok {
					t = v == 1
				} else {
					log.Warnf("[m2m] received event field is not valid, value on=%v", on)
				}
				msg := M2MMsg{Typ: "lightOn", Data: t}
				b, _ := json.Marshal(msg)
				log.Debugf("[m2m] to publish control message, topic=%q message=%s", topic, b)
				tk := client.Publish(topic, 1, false, b)
				tk.Wait()
				if tk.Error() != nil {
					log.Errorf("[m2m] publish message error, msg=%s , error: %v", b, tk.Error())
				} else {
					log.Debugf("[m2m] publish message success, topic=%q, msg=%s", topic, b)
				}
			} else {
				log.Warnf("[m2m] received event is not valid, has no field \"on\" value=%v", e)
			}
		}

	}
}

func subscribeM2M(client mqtt.Client, srcThingId string) {
	topic := m2mTopic(srcThingId)
	tk := client.Subscribe(topic, 1, func(c mqtt.Client, m mqtt.Message) {
		log.Infof("[m2m] received message: %s", m.Payload())
		var msg M2MMsg
		err := json.Unmarshal([]byte(m.Payload()), &msg)
		if err != nil {
			log.Errorf("[m2m] received invalid message, content=%s, error: %s", m.Payload(), err)
			return
		}
		if msg.Typ != "lightOn" {
			return
		}
		if on, ok := msg.Data.(bool); ok {
			lightOn(on)
		} else {
			log.Errorf("[m2m] received msg data is not a bool")
		}
	})
	tk.Wait()
	if tk.Error() != nil {
		log.Errorf("[m2m] subscription error, topic=%q, error: %v", topic, tk.Error())
	} else {
		log.Infof("[m2m] subscribe control, topic=%q", topic)
	}
}

func m2mTopic(srcThingId string) string {
	return strings.ReplaceAll(m2mTopicTmpl, "{thingId}", srcThingId)
}
