package commands

import (
	"fmt"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (sess *Session) Subscribe(cmd datatypes.Command) (string, error) {
	if err := sess.ensurePubsubClient(); err != nil {
		return "", err
	}
	topic := cmd.Args[0]
	sess.srv.pubsub.Subscribe(sess.pubsubClient, topic.String())
	subscriptionCount := sess.srv.pubsub.GetSubscriptionCountbyClient(sess.pubsubClient)
	response := []string{"subscribe", topic.String(), fmt.Sprintf("%d", subscriptionCount)}
	return resp.EncodeArray(response), nil
}

func (sess *Session) Publish(cmd datatypes.Command) (string, error) {
	topic := cmd.Args[0]
	message := cmd.Args[1]
	sess.srv.pubsub.Publish(topic.String(), message.String())
	subscriptionCount := sess.srv.pubsub.GetSubscriptionCountbyTopic(topic.String())
	return resp.EncodeInteger(int64(subscriptionCount)), nil
}

func (sess *Session) Unsubscribe(cmd datatypes.Command) (string, error) {
	if sess.pubsubClient == nil {
		return resp.EncodeError("not subscribed to any topics"), nil
	}
	topic := cmd.Args[0]
	sess.srv.pubsub.Unsubscribe(sess.pubsubClient, topic.String())
	subscriptionCount := sess.srv.pubsub.GetSubscriptionCountbyClient(sess.pubsubClient)
	response := []string{"unsubscribe", topic.String(), fmt.Sprintf("%d", subscriptionCount)}
	return resp.EncodeArray(response), nil
}
