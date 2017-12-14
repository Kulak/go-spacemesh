package tests

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/UnrulyOS/go-unruly/assert"
	"github.com/UnrulyOS/go-unruly/crypto"
	"github.com/UnrulyOS/go-unruly/log"
	"github.com/UnrulyOS/go-unruly/p2p2/keys"
	"github.com/UnrulyOS/go-unruly/p2p2/pb"
	"github.com/UnrulyOS/go-unruly/p2p2/swarm"
	"github.com/gogo/protobuf/proto"
	"testing"
)

// Basic handshake protocol data test
func TestHandshakeCoreData(t *testing.T) {

	// node 1

	port := crypto.GetRandomUInt32(1000) + 10000
	address := fmt.Sprintf("localhost:%d", port)

	priv, pub, _ := keys.GenerateKeyPair()
	node1Local, err := swarm.NewLocalNode(pub, priv, address)

	if err != nil {
		t.Error("failed to create local node1", err)
	}

	// this will be node 2 view of node 1
	node1Remote, _ := swarm.NewRemoteNode(pub.String(), address)

	// node 2

	port1 := crypto.GetRandomUInt32(1000) + 10000
	address1 := fmt.Sprintf("localhost:%d", port1)

	priv1, pub1, _ := keys.GenerateKeyPair()
	node2Local, err := swarm.NewLocalNode(pub1, priv1, address1)

	if err != nil {
		t.Error("failed to create local node2", err)
	}

	// this will be node1 view of node 2
	node2Remote, _ := swarm.NewRemoteNode(pub1.String(), address1)

	// STEP 1: Node1 generates handshake data and sends it to node2 ....
	data, session, err := swarm.GenereateHandshakeRequestData(node1Local, node2Remote)

	assert.NoErr(t, err, "expected no error")
	assert.NotNil(t, session, "expected session")
	assert.NotNil(t, data, "expected session")

	log.Info("Node 1 session data: Id:%s, AES-KEY:%s", hex.EncodeToString(session.Id()), hex.EncodeToString(session.KeyE()))

	assert.False(t, session.IsAuthenticated(), "Expected session to be not authenticated yet")

	// STEP 2: Node2 gets handshake data from node 1 and processes it to establish a session with a shared AES key
	resp, session1, err := swarm.ProcessHandshakeRequest(node2Local, node1Remote, data)

	assert.NoErr(t, err, "expected no error")
	assert.NotNil(t, session1, "expected session")
	assert.NotNil(t, resp, "expected resp data")

	log.Info("Node 2 session data: Id:%s, AES-KEY:%s", hex.EncodeToString(session1.Id()), hex.EncodeToString(session1.KeyE()))

	assert.Equal(t, string(session.Id()), string(session1.Id()), "expected agreed Id")
	assert.Equal(t, string(session.KeyE()), string(session1.KeyE()), "expected same shared AES enc key")
	assert.Equal(t, string(session.KeyM()), string(session1.KeyM()), "expected same shared AES mac key")
	assert.Equal(t, string(session.PubKey()), string(session1.PubKey()), "expected same shared secret")

	assert.True(t, session1.IsAuthenticated(), "expected session1 to be authenticated")

	// STEP 3: Node2 sends data1 back to node1.... Node 1 validates the data and sets its network session to authenticated
	err = swarm.ProcessHandshakeResponse(node1Local, node2Remote, session, resp)

	assert.True(t, session.IsAuthenticated(), "expected session to be authenticated")
	assert.NoErr(t, err, "failed to authenticate or process response")

	// test session sym enc / dec

	const msg = "hello unruly - hello unruly - hello unruly :-)"
	cipherText, err := session.Encrypt([]byte(msg))
	assert.NoErr(t, err, "expected no error")

	clearText, err := session1.Decrypt(cipherText)
	assert.NoErr(t, err, "expected no error")
	assert.True(t, bytes.Equal(clearText, []byte(msg)), "Expected enc/dec to work")

}

func TestHandshakeProtocol(t *testing.T) {

	// node 1

	port := crypto.GetRandomUInt32(1000) + 10000
	address := fmt.Sprintf("localhost:%d", port)

	priv, pub, _ := keys.GenerateKeyPair()
	node1Local, _ := swarm.NewLocalNode(pub, priv, address)
	node1Remote, _ := swarm.NewRemoteNode(pub.String(), address)

	// node 2

	port1 := crypto.GetRandomUInt32(1000) + 10000
	address1 := fmt.Sprintf("localhost:%d", port1)

	priv1, pub1, _ := keys.GenerateKeyPair()
	node2Remote, _ := swarm.NewRemoteNode(pub1.String(), address1)
	node2Local, _ := swarm.NewLocalNode(pub1, priv1, address1)

	// STEP 1: Node 1 generates handshake data and sends it to node2 ....
	data, session, err := swarm.GenereateHandshakeRequestData(node1Local, node2Remote)

	assert.NoErr(t, err, "expected no error")
	assert.NotNil(t, session, "expected session")
	assert.NotNil(t, data, "expected session")

	log.Info("Node 1 session data: Id:%s, AES-KEY:%s", hex.EncodeToString(session.Id()), hex.EncodeToString(session.KeyE()))

	assert.False(t, session.IsAuthenticated(), "expected session to be not authenticated yet")

	// encode message data
	wireFormat, err := proto.Marshal(data)

	// STEP 2: node 2 figures out if incoming message is a handshake request and handle it

	m := &pb.CommonMessageData{}
	err = proto.Unmarshal(wireFormat, m)
	assert.NoErr(t, err, "failed to unmarshal wire formatted data")

	assert.Equal(t, len(m.Payload), 0, "expected an empty payload for handshake request")

	// since payload is nil this is a handshake req or response and we can deserialize it

	data1 := &pb.HandshakeData{}
	err = proto.Unmarshal(wireFormat, data1)
	assert.NoErr(t, err, "failed to unmarshal wire formatted data to handshake data")
	assert.Equal(t, swarm.HandshakeReq, data1.Protocol, "expected this message to be a handshake req")

	// STEP 3: local node 2 handles req data and generates response

	resp, session1, err := swarm.ProcessHandshakeRequest(node2Local, node1Remote, data1)

	assert.NoErr(t, err, "expected no error")
	assert.NotNil(t, session1, "expected session")
	assert.NotNil(t, resp, "expected resp data")

	log.Info("Node 2 session data: Id:%s, AES-KEY:%s", hex.EncodeToString(session1.Id()), hex.EncodeToString(session1.KeyE()))

	assert.Equal(t, string(session.Id()), string(session1.Id()), "expected agreed Id")
	assert.Equal(t, string(session.KeyE()), string(session1.KeyE()), "expected same shared AES enc key")
	assert.Equal(t, string(session.KeyM()), string(session1.KeyM()), "expected same shared AES mac key")
	assert.Equal(t, string(session.PubKey()), string(session1.PubKey()), "expected same shared secret")

	assert.True(t, session1.IsAuthenticated(), "expected session1 to be authenticated")

	// encode message data to wire format
	wireFormat, err = proto.Marshal(resp)

	// wireFormat data sent from node 2 to node 1....

	// STEP 4: node 1 - figure out if incoming message is a handshake response

	m = &pb.CommonMessageData{}
	err = proto.Unmarshal(wireFormat, m)
	assert.NoErr(t, err, "failed to unmarshal wire formatted data")

	assert.Equal(t, len(m.Payload), 0, "expected an empty payload for handshake response")

	// since payload is nil this is a handshake req or response and we can deserialize it

	data2 := &pb.HandshakeData{}
	err = proto.Unmarshal(wireFormat, data2)
	assert.NoErr(t, err, "failed to unmarshal wire formatted data to handshake data")
	assert.Equal(t, swarm.HandshakeResp, data2.Protocol, "expected this message to be a handshake req")

	// STEP 5: Node 1 validates the data and sets its network session to authenticated

	err = swarm.ProcessHandshakeResponse(node1Local, node2Remote, session, data2)

	assert.True(t, session.IsAuthenticated(), "expected session to be authenticated")
	assert.NoErr(t, err, "failed to authenticate or process response")
}