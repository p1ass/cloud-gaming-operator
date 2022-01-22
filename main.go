package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/urfave/cli/v2"
	"google.golang.org/api/compute/v1"
)

var projectID string
var region = "asia-northeast1"
var zone = "asia-northeast1-a"

var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

func main() {

	ctx := context.Background()
	computeService, err := compute.NewService(ctx)
	if err != nil {
		log.Fatal(err)
	}

	app := &cli.App{
		Name:  "cloud-gaming-operator",
		Usage: "GCEに立てているクラウドゲーミング用のインスタンスを管理するCLIツール",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "projectID",
				Aliases:  []string{"p"},
				Required: true,
				Usage:    "GCPのプロジェクトIDを指定する",
				EnvVars:  []string{"CLOUD_GAMING_OPERATOR_PROJECT_ID"},
			},
			&cli.StringFlag{
				Name:    "region",
				Usage:   "GCPのリージョンを指定する (デフォルト: asia-northeast1)",
				EnvVars: []string{"CLOUD_GAMING_OPERATOR_REGION"},
			},
			&cli.StringFlag{
				Name:    "zone",
				Usage:   "GCPのプロジェクトIDを指定する (デフォルト: asia-northeast1-a)",
				EnvVars: []string{"CLOUD_GAMING_OPERATOR_ZONE"},
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "現在起動中のインスタンスを表示する。",
				Action: func(c *cli.Context) error {
					setGCPConfig(c)
					return listInstances(computeService)
				},
			},
			{
				Name:    "create",
				Aliases: []string{"c"},
				Usage:   "インスタンスを起動する。すでに起動している場合は何もしない。",
				Action: func(c *cli.Context) error {
					setGCPConfig(c)
					return createInstanceFromMachineImage(computeService)
				},
			},
			{
				Name:    "remove",
				Aliases: []string{"r"},
				Usage:   "マシーンイメージを作成して、インスタンスを削除する。",
				Action: func(c *cli.Context) error {
					setGCPConfig(c)
					return createMachineImageAndRemoveInstance(computeService)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func setGCPConfig(c *cli.Context) {
	projectID = c.String("projectID")
	if c.String("region") != "" {
		region = c.String("region")
	}
	if c.String("zone") != "" {
		region = c.String("zone")
	}
}

func listInstances(service *compute.Service) error {
	list, err := service.Instances.List(projectID, zone).Do()
	if err != nil {
		return err
	}

	if len(list.Items) == 0 {
		fmt.Println("起動しているインスタンスはありません")
		return nil
	}

	for _, instance := range list.Items {
		parsed, err := time.Parse(time.RFC3339, instance.LastStartTimestamp)
		if err != nil {
			return err
		}
		fmt.Printf("Name: %s, LastStart: %s\n", instance.Name, parsed.In(jst))
	}
	return nil
}

func createInstanceFromMachineImage(service *compute.Service) error {
	instanceList, err := service.Instances.List(projectID, zone).Do()
	if err != nil {
		return err
	}

	if len(instanceList.Items) != 0 {
		fmt.Println("すでに起動中です")
		return listInstances(service)
	}

	imageList, err := service.MachineImages.List(projectID).Do()
	if err != nil {
		return err
	}
	if len(imageList.Items) == 0 {
		return errors.New("マシンイメージが存在しないのでインスタンスを作成できません")
	}
	if len(imageList.Items) != 1 {
		return errors.New("マシンイメージが複数存在するので、使用するマシンイメージを特定できません")
	}
	machineImage := imageList.Items[0]

	instance := &compute.Instance{
		Name:               `instance-` + time.Now().Format("2006-01-02-15-04-05"),
		SourceMachineImage: "global/machineImages/" + machineImage.Name,
	}

	ops, err := service.Instances.Insert(projectID, zone, instance).Do()
	if err != nil {
		return err
	}
	printToJSON(ops)

	if err := waitZoneOperation(service, ops); err != nil {
		return err
	}
	return nil
}

func createMachineImageAndRemoveInstance(service *compute.Service) error {
	list, err := service.Instances.List(projectID, zone).Do()
	if err != nil {
		return err
	}

	if len(list.Items) == 0 {
		return errors.New("起動しているインスタンスはありません")
	}
	if len(list.Items) != 1 {
		return errors.New("複数台のインスタンスが起動しています。複数台インスタンスの操作は対応していません。コンソールから削除してください")
	}

	instance := list.Items[0]

	ops, err := service.Instances.Stop(projectID, zone, instance.Name).Do()
	if err != nil {
		return err
	}
	printToJSON(ops)
	if err := waitZoneOperation(service, ops); err != nil {
		return err
	}
	fmt.Printf("インスタンス %s を停止しました。\n", instance.Name)

	if err := createMachineImageAndRemoveOtherMachineImage(service, instance); err != nil {
		return err
	}

	ops, err = service.Instances.Delete(projectID, zone, instance.Name).Do()
	if err != nil {
		return err
	}
	printToJSON(ops)
	if err := waitZoneOperation(service, ops); err != nil {
		return err
	}
	fmt.Printf("インスタンス %s の削除が完了しました。\n", instance.Name)

	return nil
}

func createMachineImageAndRemoveOtherMachineImage(service *compute.Service, instance *compute.Instance) error {
	now := time.Now().In(jst)

	creatingImage := &compute.MachineImage{
		Description:      fmt.Sprintf("%sに作成したイメージです", now.Format("2006-01-02 15:04:05")),
		Name:             fmt.Sprintf("backup-%s", time.Now().Format("2006-01-02-15-04-05")),
		SatisfiesPzs:     false,
		SourceInstance:   fmt.Sprintf("projects/%s/zones/%s/instances/%s", projectID, zone, instance.Name),
		StorageLocations: []string{region},
	}

	ops, err := service.MachineImages.Insert(projectID, creatingImage).Do()
	if err != nil {
		return err
	}
	printToJSON(ops)

	if err := waitGlobalOperation(service, ops); err != nil {
		return err
	}

	fmt.Println("マシンイメージの作成が完了しました。")

	list, err := service.MachineImages.List(projectID).Do()
	if err != nil {
		return err
	}

	for _, image := range list.Items {
		// 今回作成したマシンイメージ以外は、コスト削減のため削除する
		if image.Name != creatingImage.Name {
			ops, err := service.MachineImages.Delete(projectID, image.Name).Do()
			printToJSON(ops)
			if err != nil {
				return err
			}
		}
	}

	fmt.Println("今回作成した以外のマシンイメージを削除しました")

	return nil
}

func waitGlobalOperation(service *compute.Service, ops *compute.Operation) error {
	for {
		time.Sleep(5 * time.Second)
		ops, err := service.GlobalOperations.Get(projectID, ops.Name).Do()
		if err != nil {
			return err
		}
		fmt.Printf("Status: %s, Progress: %d\n", ops.Status, ops.Progress)

		if ops.Status == "DONE" {
			break
		}
	}
	return nil
}

func waitZoneOperation(service *compute.Service, ops *compute.Operation) error {
	for {
		time.Sleep(5 * time.Second)
		ops, err := service.ZoneOperations.Get(projectID, zone, ops.Name).Do()
		if err != nil {
			return err
		}
		fmt.Printf("Status: %s, Progress: %d\n", ops.Status, ops.Progress)

		if ops.Status == "DONE" {
			break
		}
	}
	return nil
}

func printToJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "	")
	if err := enc.Encode(v); err != nil {
		log.Fatal(err)
	}
}
