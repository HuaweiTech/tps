package main_test

import (
	"fmt"

	Bbs "github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/tps/testrunner"
	"github.com/cloudfoundry/gunk/diegonats"
	"github.com/cloudfoundry/gunk/timeprovider/faketimeprovider"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"testing"
	"time"
)

var store storeadapter.StoreAdapter
var bbs *Bbs.BBS
var timeProvider *faketimeprovider.FakeTimeProvider

var tpsAddr string
var tps ifrit.Process
var runner *ginkgomon.Runner

var etcdRunner *etcdstorerunner.ETCDClusterRunner
var natsPort int
var gnatsdRunner ifrit.Process
var natsClient diegonats.NATSClient

var heartbeatInterval = 50 * time.Millisecond
var tpsBinPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	synchronizedTpsBinPath, err := gexec.Build("github.com/cloudfoundry-incubator/tps", "-race")
	Ω(err).ShouldNot(HaveOccurred())
	return []byte(synchronizedTpsBinPath)
}, func(synchronizedTpsBinPath []byte) {
	tpsBinPath = string(synchronizedTpsBinPath)
})

func TestTPS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TPS Suite")
}

var _ = BeforeEach(func() {
	tpsAddr = fmt.Sprintf("127.0.0.1:%d", uint16(1518+GinkgoParallelNode()))
	etcdPort := 5001 + GinkgoParallelNode()
	natsPort = 4001 + GinkgoParallelNode()

	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)

	store = etcdRunner.Adapter()
	timeProvider = faketimeprovider.New(time.Unix(0, 1138))
	bbs = Bbs.NewBBS(store, timeProvider, lagertest.NewTestLogger("test"))

	runner = testrunner.New(
		string(tpsBinPath),
		tpsAddr,
		[]string{fmt.Sprintf("http://127.0.0.1:%d", etcdPort)},
		[]string{fmt.Sprintf("127.0.0.1:%d", natsPort)},
		heartbeatInterval,
	)

	startAll()
})

var _ = AfterEach(func() {
	stopAll()
})

var _ = SynchronizedAfterSuite(func() {
	stopAll()
}, func() {
	gexec.CleanupBuildArtifacts()
})

func startAll() {
	etcdRunner.Start()
	gnatsdRunner, natsClient = diegonats.StartGnatsd(natsPort)
}

func stopAll() {
	if etcdRunner != nil {
		etcdRunner.Stop()
	}
	ginkgomon.Kill(gnatsdRunner)
}
