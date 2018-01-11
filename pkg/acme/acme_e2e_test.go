package acme

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/golang/mock/gomock"
	"github.com/jetstack/kube-lego/pkg/mocks"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var log = logrus.WithField("context", "test-mock")

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func setupMockedKubeLego(t *testing.T) (*mocks.MockKubeLego, *gomock.Controller) {
	ctrl := gomock.NewController(t)

	mockKL := mocks.NewMockKubeLego(ctrl)
	mockKL.EXPECT().Log().AnyTimes().Return(log)
	mockKL.EXPECT().Version().AnyTimes().Return("mocked-version")
	mockKL.EXPECT().LegoHTTPPort().AnyTimes().Return(intstr.FromInt(8181))
	mockKL.EXPECT().AcmeUser().MinTimes(1).Return(nil, errors.New("I am only mocked"))
	mockKL.EXPECT().LegoURL().MinTimes(1).Return("https://acme-staging.api.letsencrypt.org/directory")
	mockKL.EXPECT().LegoEmail().MinTimes(1).Return("kube-lego-e2e@example.com")
	mockKL.EXPECT().SaveAcmeUser(gomock.Any()).MinTimes(1).Return(nil)
	mockKL.EXPECT().LegoRsaKeySize().AnyTimes().Return(2048)
	mockKL.EXPECT().ExponentialBackoffMaxElapsedTime().MinTimes(1).Return(time.Minute * 5)
	mockKL.EXPECT().ExponentialBackoffInitialInterval().MinTimes(1).Return(time.Second * 30)
	mockKL.EXPECT().ExponentialBackoffMultiplier().MinTimes(1).Return(2.0)

	return mockKL, ctrl
}

func setupNgrok(t *testing.T) *exec.Cmd {
	command := []string{"ngrok", "http", "--bind-tls", "false", "8181"}
	cmdNgrok := exec.Command(command[0], command[1:]...)
	err := cmdNgrok.Start()
	if err != nil {
		t.Skip("Skipping e2e test as ngrok executable is not available: ", err)
		return nil
	}

	return cmdNgrok
}

func getDomain() string {
	regexDomain := regexp.MustCompile("http://([a-z0-9]+.\\.ngrok\\.io)")
	var domain string
	for {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get("http://localhost:4040/status")
		if err != nil {
			log.Warn(err)
			continue
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Warn(err)
			continue
		}

		matched := regexDomain.FindStringSubmatch(string(body))
		if matched == nil {
			continue
		}

		domain = matched[1]
		log.Infof("kube-lego domain is %s", domain)

		break
	}

	return domain
}

func obtainCertificate(t *testing.T, mockKL *mocks.MockKubeLego) {
	cmdNgrok := setupNgrok(t)
	defer cmdNgrok.Process.Kill()

	domain := getDomain()

	stopCh := make(chan struct{})
	a := New(mockKL)
	go a.RunServer(stopCh)

	log.Infof("trying to obtain a certificate for the domain")
	a.ObtainCertificate([]string{domain})

}

func TestAcme_E2E(t *testing.T) {
	mockKL, mockCtrl := setupMockedKubeLego(t)
	defer mockCtrl.Finish()

	mockKL.EXPECT().AcmeUser().MinTimes(1).Return(nil, errors.New("I am only mocked"))
	mockKL.EXPECT().LegoURL().MinTimes(1).Return("https://acme-staging.api.letsencrypt.org/directory")
	mockKL.EXPECT().LegoEmail().MinTimes(1).Return("kube-lego-e2e@example.com")
	mockKL.EXPECT().SaveAcmeUser(gomock.Any()).MinTimes(1).Return(nil)

	obtainCertificate(t, mockKL)
}
