package p2p

import (
	"context"
	"errors"
	"fmt"
	"github.com/spacemeshos/go-spacemesh/p2p/service"
	"github.com/spacemeshos/go-spacemesh/rand"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
	"net/http"
	_ "net/http/pprof"
	"sync/atomic"
	"testing"
	"time"
)

//func (its *IntegrationTestSuite) Test_SendingMessage() {
//	exProto := RandString(10)
//	exMsg := RandString(10)
//
//	node1 := its.Instances[0]
//	node2 := its.Instances[1]
//
//	_ = node1.RegisterDirectProtocol(exProto)
//	ch2 := node2.RegisterDirectProtocol(exProto)
//	conn, err := node1.cPool.GetConnection(node2.network.LocalAddr().String(), node2.lNode.PublicKey())
//	require.NoError(its.T(), err)
//	err = node1.SendMessage(node2.LocalNode().NodeInfo.PublicKey(), exProto, []byte(exMsg))
//	require.NoError(its.T(), err)
//
//	tm := time.After(1 * time.Second)
//
//	select {
//	case gotmessage := <-ch2:
//		if string(gotmessage.Bytes()) != exMsg {
//			its.T().Fatal("got wrong message")
//		}
//	case <-tm:
//		its.T().Fatal("failed to deliver message within second")
//	}
//	conn.Close()
//}

func (its *IntegrationTestSuite) Test_Gossiping() {

	msgChans := make([]chan service.GossipMessage, 0)
	exProto := RandString(10)

	its.ForAll(func(idx int, s NodeTestInstance) error {
		msgChans = append(msgChans, s.RegisterGossipProtocol(exProto))
		return nil
	}, nil)

	ctx, _ := context.WithTimeout(context.Background(), time.Second*100)
	errg, ctx := errgroup.WithContext(ctx)
	MSGS := 100
	numgot := int32(0)
	all := time.Now()
	for i := 0; i < MSGS; i++ {
		msg := []byte(RandString(108692))
		rnd := rand.Int31n(int32(len(its.Instances)))
		_ = its.Instances[rnd].Broadcast(exProto, []byte(msg))
		for _, mc := range msgChans {
			ctx := ctx
			mc := mc
			numgot := &numgot
			errg.Go(func() error {
				select {
				case got := <-mc:
					got.ReportValidation(exProto)
					atomic.AddInt32(numgot, 1)
					return nil
				case <-ctx.Done():
					return errors.New("timed out")
				}
			})
		}
	}
	fmt.Println("########## took ", time.Since(all), " too send all")

	errs := errg.Wait()
	fmt.Println("########## took ", time.Since(all), " too get all")
	its.T().Log(errs)
	its.NoError(errs)
	its.Equal(int(numgot), (its.BootstrappedNodeCount+its.BootstrapNodesCount)*MSGS)
}

func Test_ReallySmallP2PIntegrationSuite(t *testing.T) {
	go func () { http.ListenAndServe(":6060", nil )}()

	if testing.Short() {
		t.Skip()
	}

	//log.D ebugMode(true)

	s := new(IntegrationTestSuite)

	s.BootstrappedNodeCount = 100
	s.BootstrapNodesCount = 1
	s.NeighborsCount = 8

	suite.Run(t, s)
}

func Test_SmallP2PIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	s := new(IntegrationTestSuite)

	s.BootstrappedNodeCount = 70
	s.BootstrapNodesCount = 1
	s.NeighborsCount = 8

	suite.Run(t, s)
}

func Test_BigP2PIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	s := new(IntegrationTestSuite)

	s.BootstrappedNodeCount = 100
	s.BootstrapNodesCount = 3
	s.NeighborsCount = 5

	suite.Run(t, s)
}
