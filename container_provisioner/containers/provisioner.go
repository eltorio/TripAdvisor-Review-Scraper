package containers

import (
	"container_provisioner/utils"
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

var (
	ctx = context.Background()
)

// Provision creates a container, runs it, tails the log and wait for it to exit, and export the file name
func Provision(filePrefix string, uploadIdentifier string, hotelUrl string) {

	// Get the hotel name from the URL
	hotelName := utils.GetHotelNameFromURL(hotelUrl)

	// Connect to the Docker daemon
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.ErrorHandler(err)
	defer cli.Close()

	// Pull the image => Disabled for now
	// reader, err := cli.ImagePull(ctx, "ghcr.io/algo7/tripadvisor-review-scraper/scrap:latest", types.ImagePullOptions{})
	// utils.ErrorHandler(err)
	// defer reader.Close()

	// Print the progress of the image pull
	// _, err = io.Copy(os.Stdout, reader)
	// utils.ErrorHandler(err)

	// Create the container. Container.ID contains the ID of the container
	Container, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: "ghcr.io/algo7/tripadvisor-review-scraper/scrap:latest",
			Env: []string{
				"CONCURRENCY=1",
				"SCRAPE_MODE=HOTEL",
				"HOTEL_NAME=" + hotelName,
				"IS_PROVISIONER=true",
				"HOTEL_URL=" + hotelUrl,
			},
		},
		&container.HostConfig{
			AutoRemove: false, // Cant set to true otherwise the container got deleted before copying the file
		},
		nil, // NetworkConfig
		nil, // Platform
		"",  // Container name
	)
	utils.ErrorHandler(err)

	// Start the container
	err = cli.ContainerStart(ctx, Container.ID, types.ContainerStartOptions{})
	utils.ErrorHandler(err)

	// Log tailing disabled for now
	// // Print the logs of the container
	// out, err := cli.ContainerLogs(ctx, Container.ID, types.ContainerLogsOptions{ShowStdout: true, Follow: true})
	// utils.ErrorHandler(err)

	// // Docker log uses multiplexed streams to send stdout and stderr in the connection. This function separates them
	// _, err = stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	// utils.ErrorHandler(err)

	// Wait for the container to exit
	statusCh, errCh := cli.ContainerWait(ctx, Container.ID, container.WaitConditionNotRunning)

	// ContainerWait returns 2 channels. One for the status and one for the error
	select {
	case err := <-errCh:
		utils.ErrorHandler(err)
	case <-statusCh:
	}

	// The file path in the container
	filePathInContainer := fmt.Sprintf("/puppeteer/reviews/0_%s.csv", hotelName)

	// Read the file from the container as a reader interface of a tar stream
	fileReader, _, err := cli.CopyFromContainer(ctx, Container.ID, filePathInContainer)
	utils.ErrorHandler(err)

	// Write the file to the host
	exportedFileName := utils.WriteToFileFromTarStream(filePrefix, fileReader)

	// Read the exported csv file
	file := utils.ReadFromFile(exportedFileName)

	// Upload the file to R2
	utils.R2UploadObject(exportedFileName, uploadIdentifier, file)

	// Remove the container
	err = cli.ContainerRemove(ctx, Container.ID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	utils.ErrorHandler(err)

}

// CountRunningContainer lists the number of running containers
func CountRunningContainer() int {

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.ErrorHandler(err)
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All: false, // Only running containers
	})
	utils.ErrorHandler(err)

	return len(containers)
}
