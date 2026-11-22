// Command export renders one project onto a video file.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	videxport "mygo-uke/internal/export"
	"mygo-uke/internal/project"
)

func main() {
	projectPath := flag.String("project", "", "project.json path")
	source := flag.String("source", "", "source video path")
	output := flag.String("out", "out.mp4", "output mp4 path")
	orientation := flag.Int("orientation", 0, "0 vertical, 1 horizontal")
	flag.Parse()
	if *projectPath == "" || *source == "" {
		log.Fatal("usage: export -project project.json -source video.mp4 -out out.mp4")
	}

	p, err := project.Load(*projectPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	last := -1.0
	err = videxport.Run(
		ctx, p, *source, *output, *orientation,
		func(value float64) {
			percent := int(value * 100)
			if percent != int(last*100) {
				fmt.Printf("%d%%\n", percent)
				last = value
			}
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(*output)
}
