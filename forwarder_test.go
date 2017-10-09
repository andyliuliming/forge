package forge_test

import (
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"

	"io"

	"io/ioutil"

	"github.com/onsi/gomega/gbytes"
	"github.com/sclevine/cflocal/engine"
	"github.com/sclevine/cflocal/fixtures"
	. "github.com/sclevine/cflocal/local"
	"github.com/sclevine/cflocal/local/mocks"
	"github.com/sclevine/cflocal/service"
)

var _ = Describe("Forwarder", func() {
	var (
		forwarder        *Forwarder
		mockCtrl         *gomock.Controller
		mockEngine       *mocks.MockEngine
		mockNetContainer *mocks.MockContainer
		mockContainer    *mocks.MockContainer
		logs             *gbytes.Buffer
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockEngine = mocks.NewMockEngine(mockCtrl)
		mockNetContainer = mocks.NewMockContainer(mockCtrl)
		mockContainer = mocks.NewMockContainer(mockCtrl)
		logs = gbytes.NewBuffer()

		forwarder = &Forwarder{
			StackVersion: "some-stack-version",
			Logs:         logs,
			Engine:       mockEngine,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("#Forward", func() {
		It("should configure service tunnels and general app networking", func() {
			mockHealth := make(<-chan string)
			waiter := make(chan time.Time)
			codeIdx := 0
			config := &ForwardConfig{
				AppName: "some-app",
				SSHPass: engine.NewStream(mockReadCloser{Value: "some-sshpass"}, 300),
				Color:   percentColor,
				ForwardConfig: &service.ForwardConfig{
					Host: "some-ssh-host",
					Port: "some-port",
					User: "some-user",
					Code: func() (string, error) {
						codeIdx++
						return fmt.Sprintf("some-code-%d", codeIdx), nil
					},
					Forwards: []service.Forward{
						{
							Name: "some-name",
							From: "some-from",
							To:   "some-to",
						},
						{
							Name: "some-other-name",
							From: "some-other-from",
							To:   "some-other-to",
						},
					},
				},
				HostIP:   "some-ip",
				HostPort: "400",
				Wait:     waiter,
			}
			mockEngine.EXPECT().NewContainer(gomock.Any(), gomock.Any()).Do(func(config *container.Config, hostConfig *container.HostConfig) {
				Expect(config.Hostname).To(Equal("cflocal"))
				Expect(config.User).To(Equal("vcap"))
				Expect(config.ExposedPorts).To(HaveLen(1))
				_, hasPort := config.ExposedPorts["8080/tcp"]
				Expect(hasPort).To(BeTrue())
				Expect(config.Image).To(Equal("cloudfoundry/cflinuxfs2:some-stack-version"))
				Expect(config.Entrypoint).To(Equal(strslice.StrSlice{
					"tail", "-f", "/dev/null",
				}))
				Expect(hostConfig.PortBindings).To(HaveLen(1))
				Expect(hostConfig.PortBindings["8080/tcp"]).To(Equal([]nat.PortBinding{{HostIP: "some-ip", HostPort: "400"}}))
				Expect(hostConfig.NetworkMode).To(BeEmpty())
			}).Return(mockNetContainer, nil)

			background := mockNetContainer.EXPECT().Background()
			mockNetContainer.EXPECT().ID().Return("some-id").AnyTimes()

			mockEngine.EXPECT().NewContainer(gomock.Any(), gomock.Any()).Do(func(config *container.Config, hostConfig *container.HostConfig) {
				Expect(config.User).To(Equal("vcap"))
				Expect(config.ExposedPorts).To(BeEmpty())
				Expect(config.Healthcheck).To(Equal(&container.HealthConfig{
					Test:     []string{"CMD", "test", "-f", "/tmp/healthy"},
					Interval: time.Second,
					Retries:  30,
				}))
				Expect(config.Image).To(Equal("cloudfoundry/cflinuxfs2:some-stack-version"))
				Expect(config.Entrypoint).To(Equal(strslice.StrSlice{
					"/bin/bash", "-c", fixtures.ForwardScript(),
				}))
				Expect(hostConfig.PortBindings).To(BeEmpty())
				Expect(hostConfig.NetworkMode).To(Equal(container.NetworkMode("container:some-id")))
			}).Return(mockContainer, nil).After(background)

			mockContainer.EXPECT().CopyTo(config.SSHPass, "/usr/bin/sshpass")
			mockContainer.EXPECT().HealthCheck().Return(mockHealth)

			health, done, id, err := forwarder.Forward(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(health).To(Equal(mockHealth))
			Expect(id).To(Equal("some-id"))

			gomock.InOrder(
				mockContainer.EXPECT().CopyTo(gomock.Any(), "/tmp/ssh-code").Do(func(stream engine.Stream, _ string) {
					defer GinkgoRecover()
					defer stream.Close()
					Expect(ioutil.ReadAll(stream)).To(Equal([]byte("some-code-1")))
				}),
				mockContainer.EXPECT().Start("[some-app tunnel] % ", gomock.Any(), nil).Do(func(_ string, output io.Writer, _ <-chan time.Time) {
					fmt.Fprint(output, "start-1")
				}).Return(int64(100), nil),
				mockContainer.EXPECT().CopyTo(gomock.Any(), "/tmp/ssh-code").Do(func(stream engine.Stream, _ string) {
					defer GinkgoRecover()
					defer stream.Close()
					Expect(ioutil.ReadAll(stream)).To(Equal([]byte("some-code-2")))
				}),
				mockContainer.EXPECT().Start("[some-app tunnel] % ", gomock.Any(), nil).Do(func(_ string, output io.Writer, _ <-chan time.Time) {
					fmt.Fprint(output, "start-2")
					done()
				}).Return(int64(200), nil),
				mockContainer.EXPECT().Close(),
				mockNetContainer.EXPECT().Close(),
			)

			waiter <- time.Time{}
			waiter <- time.Time{}

			Expect(logs).To(gbytes.Say(`start-1\[some-app tunnel\] % Exited with status: 100`))
			Expect(logs).To(gbytes.Say("start-2"))
			Expect(logs).NotTo(gbytes.Say(`\[some-app tunnel\] % Exited with status: 200`))
		})
	})
})
