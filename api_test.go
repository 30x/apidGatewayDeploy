package apiGatewayDeploy

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"net/url"
	"encoding/json"
	"io/ioutil"
	"time"
	"bytes"
)

var _ = Describe("api", func() {

	Context("GET /deployments", func() {

		It("should get an empty array if no deployments", func() {

			uri, err := url.Parse(testServer.URL)
			uri.Path = deploymentsEndpoint

			res, err := http.Get(uri.String())
			Expect(err).ShouldNot(HaveOccurred())
			defer res.Body.Close()
			Expect(res.StatusCode).Should(Equal(http.StatusNotFound))
		})

		It("should get current deployments", func() {

			deploymentID := "api_get_current"
			insertTestDeployment(testServer, deploymentID)

			uri, err := url.Parse(testServer.URL)
			uri.Path = deploymentsEndpoint

			res, err := http.Get(uri.String())
			Expect(err).ShouldNot(HaveOccurred())
			defer res.Body.Close()

			var depRes ApiDeploymentResponse
			body, err := ioutil.ReadAll(res.Body)
			Expect(err).ShouldNot(HaveOccurred())
			json.Unmarshal(body, &depRes)

			Expect(len(depRes)).To(Equal(1))

			dep := depRes[0]

			Expect(dep.ID).To(Equal(deploymentID))
			Expect(dep.ScopeId).To(Equal(deploymentID))
			Expect(dep.DisplayName).To(Equal(deploymentID))

			var config bundleConfigJson

			err = json.Unmarshal(dep.ConfigJson, &config)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(config.Name).To(Equal("/bundles/1"))

			err = json.Unmarshal(dep.BundleConfigJson, &config)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(config.Name).To(Equal("/bundles/1"))
		})

		It("should get 304 for no change", func() {

			deploymentID := "api_no_change"
			insertTestDeployment(testServer, deploymentID)

			uri, err := url.Parse(testServer.URL)
			uri.Path = deploymentsEndpoint
			res, err := http.Get(uri.String())
			Expect(err).ShouldNot(HaveOccurred())
			defer res.Body.Close()

			req, err := http.NewRequest("GET", uri.String(), nil)
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("If-None-Match", res.Header.Get("etag"))

			res, err = http.DefaultClient.Do(req)
			Expect(err).ShouldNot(HaveOccurred())
			defer res.Body.Close()
			Expect(res.StatusCode).To(Equal(http.StatusNotModified))
		})

		It("should get empty set after blocking if no deployments", func() {

			uri, err := url.Parse(testServer.URL)
			uri.Path = deploymentsEndpoint

			query := uri.Query()
			query.Add("block", "1")
			uri.RawQuery = query.Encode()
			res, err := http.Get(uri.String())
			Expect(err).ShouldNot(HaveOccurred())
			defer res.Body.Close()

			Expect(res.StatusCode).Should(Equal(http.StatusOK))

		})

		It("should get new deployment after blocking", func(done Done) {

			deploymentID := "api_get_current_blocking"
			insertTestDeployment(testServer, deploymentID)
			uri, err := url.Parse(testServer.URL)
			uri.Path = deploymentsEndpoint
			res, err := http.Get(uri.String())
			Expect(err).ShouldNot(HaveOccurred())
			defer res.Body.Close()

			deploymentID = "api_get_current_blocking2"
			go func() {
				defer GinkgoRecover()

				query := uri.Query()
				query.Add("block", "1")
				uri.RawQuery = query.Encode()
				req, err := http.NewRequest("GET", uri.String(), nil)
				req.Header.Add("Content-Type", "application/json")
				req.Header.Add("If-None-Match", res.Header.Get("etag"))

				res, err := http.DefaultClient.Do(req)
				Expect(err).ShouldNot(HaveOccurred())
				defer res.Body.Close()
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				var depRes ApiDeploymentResponse
				body, err := ioutil.ReadAll(res.Body)
				Expect(err).ShouldNot(HaveOccurred())
				json.Unmarshal(body, &depRes)

				Expect(len(depRes)).To(Equal(2))

				dep := depRes[1]

				Expect(dep.ID).To(Equal(deploymentID))
				Expect(dep.ScopeId).To(Equal(deploymentID))
				Expect(dep.DisplayName).To(Equal(deploymentID))

				close(done)
			}()

			time.Sleep(250 * time.Millisecond) // give api call above time to block
			insertTestDeployment(testServer, deploymentID)
			deploymentsChanged<- deploymentID
		})

		It("should get 304 after blocking if no new deployment", func() {

			deploymentID := "api_no_change_blocking"
			insertTestDeployment(testServer, deploymentID)
			uri, err := url.Parse(testServer.URL)
			uri.Path = deploymentsEndpoint
			res, err := http.Get(uri.String())
			Expect(err).ShouldNot(HaveOccurred())
			defer res.Body.Close()

			query := uri.Query()
			query.Add("block", "1")
			uri.RawQuery = query.Encode()
			req, err := http.NewRequest("GET", uri.String(), nil)
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("If-None-Match", res.Header.Get("etag"))

			res, err = http.DefaultClient.Do(req)
			Expect(err).ShouldNot(HaveOccurred())
			defer res.Body.Close()
			Expect(res.StatusCode).To(Equal(http.StatusNotModified))
		})
	})

	Context("POST /deployments", func() {

		It("should return BadRequest for invalid request", func() {

			uri, err := url.Parse(testServer.URL)
			uri.Path = deploymentsEndpoint

			deploymentResult := apiDeploymentResults{
				apiDeploymentResult{
				},
			}
			payload, err := json.Marshal(deploymentResult)
			Expect(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", uri.String(), bytes.NewReader(payload))
			req.Header.Add("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			defer resp.Body.Close()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).Should(Equal(http.StatusBadRequest))
		})

		It("should ignore deployments that can't be found", func() {

			deploymentID := "api_missing_deployment"

			uri, err := url.Parse(testServer.URL)
			uri.Path = deploymentsEndpoint

			deploymentResult := apiDeploymentResults{
				apiDeploymentResult{
					ID: deploymentID,
					Status: RESPONSE_STATUS_SUCCESS,
				},
			}
			payload, err := json.Marshal(deploymentResult)
			Expect(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", uri.String(), bytes.NewReader(payload))
			req.Header.Add("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			defer resp.Body.Close()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).Should(Equal(http.StatusOK))
		})

		It("should mark a deployment as successful", func() {

			db := getDB()
			deploymentID := "api_mark_deployed"
			insertTestDeployment(testServer, deploymentID)

			uri, err := url.Parse(testServer.URL)
			uri.Path = deploymentsEndpoint

			deploymentResult := apiDeploymentResults{
				apiDeploymentResult{
					ID: deploymentID,
					Status: RESPONSE_STATUS_SUCCESS,
				},
			}
			payload, err := json.Marshal(deploymentResult)
			Expect(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", uri.String(), bytes.NewReader(payload))
			req.Header.Add("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			defer resp.Body.Close()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).Should(Equal(http.StatusOK))

			var deployStatus string
			err = db.QueryRow("SELECT deploy_status FROM deployments WHERE id=?", deploymentID).
				Scan(&deployStatus)
			Expect(deployStatus).Should(Equal(RESPONSE_STATUS_SUCCESS))
		})

		It("should mark a deployment as failed", func() {

			db := getDB()
			deploymentID := "api_mark_failed"
			insertTestDeployment(testServer, deploymentID)

			uri, err := url.Parse(testServer.URL)
			uri.Path = deploymentsEndpoint

			deploymentResult := apiDeploymentResults{
				apiDeploymentResult{
					ID: deploymentID,
					Status: RESPONSE_STATUS_FAIL,
					ErrorCode: 100,
					Message: "Some error message",
				},
			}
			payload, err := json.Marshal(deploymentResult)
			Expect(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", uri.String(), bytes.NewReader(payload))
			req.Header.Add("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			defer resp.Body.Close()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).Should(Equal(http.StatusOK))

			var deployStatus, deploy_error_message string
			var deploy_error_code int
			err = db.QueryRow(`
			SELECT deploy_status, deploy_error_code, deploy_error_message
			FROM deployments
			WHERE id=?`, deploymentID).Scan(&deployStatus, &deploy_error_code, &deploy_error_message)
			Expect(deployStatus).Should(Equal(RESPONSE_STATUS_FAIL))
			Expect(deploy_error_code).Should(Equal(100))
			Expect(deploy_error_message).Should(Equal("Some error message"))
		})
	})
})

func insertTestDeployment(testServer *httptest.Server, deploymentID string) {

	uri, err := url.Parse(testServer.URL)
	Expect(err).ShouldNot(HaveOccurred())

	uri.Path = "/bundles/1"
	bundleUri := uri.String()
	bundle := bundleConfigJson{
		Name: uri.Path,
		URI: bundleUri,
		ChecksumType: "crc-32",
	}
	bundle.Checksum = testGetChecksum(bundle.ChecksumType, bundleUri)
	bundleJson, err := json.Marshal(bundle)
	Expect(err).ShouldNot(HaveOccurred())

	tx, err := getDB().Begin()
	Expect(err).ShouldNot(HaveOccurred())

	dep := DataDeployment{
		ID: deploymentID,
		BundleConfigID: deploymentID,
		ApidClusterID: deploymentID,
		DataScopeID: deploymentID,
		BundleConfigJSON: string(bundleJson),
		ConfigJSON: string(bundleJson),
		Status: "",
		Created: "",
		CreatedBy: "",
		Updated: "",
		UpdatedBy: "",
		BundleName: deploymentID,
		BundleURI: "",
		BundleChecksum: "",
		BundleChecksumType: "",
		LocalBundleURI: "x",
	}

	err = InsertDeployment(tx, dep)
	Expect(err).ShouldNot(HaveOccurred())

	err = tx.Commit()
	Expect(err).ShouldNot(HaveOccurred())
}
