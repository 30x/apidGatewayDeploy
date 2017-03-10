package apiGatewayDeploy

import (
	"encoding/json"
	"net/url"

	"net/http/httptest"

	"net/http"

	"github.com/30x/apid-core"
	"github.com/apigee-labs/transicator/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("listener", func() {

	Context("ApigeeSync snapshot event", func() {

		It("should set DB and process", func(done Done) {

			deploymentID := "listener_test_1"

			uri, err := url.Parse(testServer.URL)
			Expect(err).ShouldNot(HaveOccurred())

			uri.Path = "/bundles/1"
			bundleUri := uri.String()
			bundle1 := bundleConfigJson{
				Name:         uri.Path,
				URI:          bundleUri,
				ChecksumType: "crc-32",
			}
			bundle1.Checksum = testGetChecksum(bundle1.ChecksumType, bundleUri)
			bundle1Json, err := json.Marshal(bundle1)
			Expect(err).ShouldNot(HaveOccurred())

			row := common.Row{}
			row["id"] = &common.ColumnVal{Value: deploymentID}
			row["bundle_config_json"] = &common.ColumnVal{Value: string(bundle1Json)}

			var event = common.Snapshot{
				SnapshotInfo: "test",
				Tables: []common.Table{
					{
						Name: DEPLOYMENT_TABLE,
						Rows: []common.Row{row},
					},
				},
			}

			var listener = make(chan deploymentsResult)
			addSubscriber <- listener

			apid.Events().Emit(APIGEE_SYNC_EVENT, &event)

			result := <-listener
			Expect(result.err).ToNot(HaveOccurred())

			// from event
			Expect(len(result.deployments)).To(Equal(1))
			d := result.deployments[0]

			Expect(d.ID).To(Equal(deploymentID))
			Expect(d.BundleName).To(Equal(bundle1.Name))
			Expect(d.BundleURI).To(Equal(bundle1.URI))

			// from db
			deployments, err := getReadyDeployments()
			Expect(err).ShouldNot(HaveOccurred())

			Expect(len(deployments)).To(Equal(1))
			d = deployments[0]

			Expect(d.ID).To(Equal(deploymentID))
			Expect(d.BundleName).To(Equal(bundle1.Name))
			Expect(d.BundleURI).To(Equal(bundle1.URI))

			close(done)
		})

		It("should process unready on existing db startup event", func(done Done) {

			deploymentID := "startup_test"

			uri, err := url.Parse(testServer.URL)
			Expect(err).ShouldNot(HaveOccurred())

			uri.Path = "/bundles/1"
			bundleUri := uri.String()
			bundle := bundleConfigJson{
				Name:         uri.Path,
				URI:          bundleUri,
				ChecksumType: "crc-32",
			}
			bundle.Checksum = testGetChecksum(bundle.ChecksumType, bundleUri)

			dep := DataDeployment{
				ID:                 deploymentID,
				DataScopeID:        deploymentID,
				BundleURI:          bundle.URI,
				BundleChecksum:     bundle.Checksum,
				BundleChecksumType: bundle.ChecksumType,
			}

			// init without info == startup on existing DB
			var snapshot = common.Snapshot{
				SnapshotInfo: "test",
				Tables:       []common.Table{},
			}

			db, err := data.DBVersion(snapshot.SnapshotInfo)
			if err != nil {
				log.Panicf("Unable to access database: %v", err)
			}

			err = InitDB(db)
			if err != nil {
				log.Panicf("Unable to initialize database: %v", err)
			}

			tx, err := db.Begin()
			Expect(err).ShouldNot(HaveOccurred())

			err = InsertDeployment(tx, dep)
			Expect(err).ShouldNot(HaveOccurred())

			err = tx.Commit()
			Expect(err).ShouldNot(HaveOccurred())

			var listener = make(chan deploymentsResult)
			addSubscriber <- listener

			apid.Events().Emit(APIGEE_SYNC_EVENT, &snapshot)

			result := <-listener
			Expect(result.err).ShouldNot(HaveOccurred())

			Expect(len(result.deployments)).To(Equal(1))
			d := result.deployments[0]

			Expect(d.ID).To(Equal(deploymentID))
			close(done)
		})

		It("should send deployment statuses on existing db startup event", func(done Done) {

			successDep := DataDeployment{
				ID:                 "success",
				LocalBundleURI:     "x",
				DeployStatus:       RESPONSE_STATUS_SUCCESS,
				DeployErrorCode:    1,
				DeployErrorMessage: "message",
			}

			failDep := DataDeployment{
				ID:                 "fail",
				LocalBundleURI:     "x",
				DeployStatus:       RESPONSE_STATUS_FAIL,
				DeployErrorCode:    1,
				DeployErrorMessage: "message",
			}

			blankDep := DataDeployment{
				ID:             "blank",
				LocalBundleURI: "x",
			}

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()

				var results apiDeploymentResults
				err := json.NewDecoder(r.Body).Decode(&results)
				Expect(err).ToNot(HaveOccurred())

				Expect(results).To(HaveLen(2))

				Expect(results).To(ContainElement(apiDeploymentResult{
					ID:        successDep.ID,
					Status:    successDep.DeployStatus,
					ErrorCode: successDep.DeployErrorCode,
					Message:   successDep.DeployErrorMessage,
				}))
				Expect(results).To(ContainElement(apiDeploymentResult{
					ID:        failDep.ID,
					Status:    failDep.DeployStatus,
					ErrorCode: failDep.DeployErrorCode,
					Message:   failDep.DeployErrorMessage,
				}))

				close(done)
			}))

			var err error
			apiServerBaseURI, err = url.Parse(ts.URL)
			Expect(err).NotTo(HaveOccurred())

			// init without info == startup on existing DB
			var snapshot = common.Snapshot{
				SnapshotInfo: "test",
				Tables:       []common.Table{},
			}

			db, err := data.DBVersion(snapshot.SnapshotInfo)
			if err != nil {
				log.Panicf("Unable to access database: %v", err)
			}

			err = InitDB(db)
			if err != nil {
				log.Panicf("Unable to initialize database: %v", err)
			}

			tx, err := db.Begin()
			Expect(err).ShouldNot(HaveOccurred())

			err = InsertDeployment(tx, successDep)
			Expect(err).ShouldNot(HaveOccurred())
			err = InsertDeployment(tx, failDep)
			Expect(err).ShouldNot(HaveOccurred())
			err = InsertDeployment(tx, blankDep)
			Expect(err).ShouldNot(HaveOccurred())

			err = tx.Commit()
			Expect(err).ShouldNot(HaveOccurred())

			apid.Events().Emit(APIGEE_SYNC_EVENT, &snapshot)
		})
	})

	Context("ApigeeSync change event", func() {

		It("add event should add a deployment", func(done Done) {

			deploymentID := "add_test_1"

			uri, err := url.Parse(testServer.URL)
			Expect(err).ShouldNot(HaveOccurred())

			uri.Path = "/bundles/1"
			bundleUri := uri.String()
			bundle := bundleConfigJson{
				Name:         uri.Path,
				URI:          bundleUri,
				ChecksumType: "crc-32",
			}
			bundle.Checksum = testGetChecksum(bundle.ChecksumType, bundleUri)
			bundle1Json, err := json.Marshal(bundle)
			Expect(err).ShouldNot(HaveOccurred())

			row := common.Row{}
			row["id"] = &common.ColumnVal{Value: deploymentID}
			row["bundle_config_json"] = &common.ColumnVal{Value: string(bundle1Json)}

			var event = common.ChangeList{
				Changes: []common.Change{
					{
						Operation: common.Insert,
						Table:     DEPLOYMENT_TABLE,
						NewRow:    row,
					},
				},
			}

			var listener = make(chan deploymentsResult)
			addSubscriber <- listener

			apid.Events().Emit(APIGEE_SYNC_EVENT, &event)

			// wait for event to propagate
			result := <-listener
			Expect(result.err).ShouldNot(HaveOccurred())

			deployments, err := getReadyDeployments()
			Expect(err).ShouldNot(HaveOccurred())

			Expect(len(deployments)).To(Equal(1))
			d := deployments[0]

			Expect(d.ID).To(Equal(deploymentID))
			Expect(d.BundleName).To(Equal(bundle.Name))
			Expect(d.BundleURI).To(Equal(bundle.URI))

			close(done)
		})

		It("delete event should delete a deployment", func(done Done) {

			deploymentID := "delete_test_1"

			tx, err := getDB().Begin()
			Expect(err).ShouldNot(HaveOccurred())
			dep := DataDeployment{
				ID:             deploymentID,
				LocalBundleURI: "whatever",
			}
			err = InsertDeployment(tx, dep)
			Expect(err).ShouldNot(HaveOccurred())
			err = tx.Commit()
			Expect(err).ShouldNot(HaveOccurred())

			row := common.Row{}
			row["id"] = &common.ColumnVal{Value: deploymentID}

			var event = common.ChangeList{
				Changes: []common.Change{
					{
						Operation: common.Delete,
						Table:     DEPLOYMENT_TABLE,
						OldRow:    row,
					},
				},
			}

			var listener = make(chan deploymentsResult)
			addSubscriber <- listener

			apid.Events().Emit(APIGEE_SYNC_EVENT, &event)

			<-listener

			deployments, err := getReadyDeployments()
			Expect(err).ShouldNot(HaveOccurred())

			Expect(len(deployments)).To(Equal(0))

			close(done)
		})
	})
})
