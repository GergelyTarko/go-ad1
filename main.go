package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"regexp"
	"strings"
)

type AD1Item struct {
	Id       int64
	Type     uint32
	Parent   int
	Filename string
	Metadata map[uint32](map[uint32]string)
	Content  []byte
}

type AD1Directory struct {
	Items []AD1Item
}

type AD1ReaderController struct {
	output string
	f      *os.File
	margin int
	size   int64
}

func (a *AD1Directory) GetItemPath(item *AD1Item) string {
	path := ""
	if item.Parent == 0 {
		path = item.Filename
	} else {
		var parent *AD1Item = nil
		for _, s := range a.Items {
			if s.Id == int64(item.Parent) {
				parent = &s
				break
			}
		}
		if parent != nil {
			path = a.GetItemPath(parent) + "/" + item.Filename
		} else {
			fmt.Println("Parent not found") // Should not be possible but w/e
		}
	}
	return path
}

func (a *AD1ReaderController) Init(source string, output string) {
	a.margin = 512
	a.output = output
	var err error = nil
	a.f, err = os.Open(source)
	if err != nil {
		fmt.Println("[ERROR] [OPEN]", err)
		return
	}

	stats, statsErr := a.f.Stat()
	if statsErr != nil {
		fmt.Println("[ERROR] [STAT]", statsErr)
		return
	}

	a.size = stats.Size()

	res := a.readHeader()
	if res == false {
		fmt.Println("[ERROR] Failed to read header")
		return
	}
}

func (a *AD1ReaderController) ReadContent(proc func(item AD1Item)) {
	a.readContent(proc)
}

func (a *AD1ReaderController) ReadToDirectory(directory *AD1Directory) {
	a.readContent(func(item AD1Item) {
		directory.Items = append(directory.Items, item)
	})
}

func (a *AD1ReaderController) Close() {
	a.f.Close()
}

// func (a AD1ReaderController) glob(path string) []string {
// 	if !strings.HasSuffix(path, ".ad1") {
// 		fmt.Println("[ERROR] Invalid file format")
// 		return []string{}
// 	}
// 	return []string{path}
// }

func (a *AD1ReaderController) read(number int) []byte {
	bytes := make([]byte, number)
	_, err := a.f.Read(bytes)
	if err != nil {
		fmt.Println("[ERROR] [read]", err)
	}
	return bytes
}

func (a *AD1ReaderController) Read(length int) []byte {
	data := a.read(length)
	// data := lastRead

	// for len(data) < length && len(a.paths) > 0 {
	// 	if len(lastRead) == 0 {

	// 	}
	// 	lastRead = a.read(length - len(data))
	// 	data = append(data, lastRead...)
	// }

	if len(data) < length {
		fmt.Println("[ERROR] [READ] Incomplete read")
	}

	return data
}

func (a *AD1ReaderController) readHeader() bool {
	fmt.Println("Reading header...")
	a.Read(a.margin) // Margin
	a.Read(16)       // Signature
	version := binary.LittleEndian.Uint32(a.read(4))
	fmt.Println("- Version:", version)
	if version != 3 && version != 4 {
		fmt.Println("Invalid version", version)
		return false
	}
	a.Read(4) // ?
	binary.LittleEndian.Uint32(a.Read(4))
	binary.LittleEndian.Uint64(a.Read(8))
	imageHeaderLength2 := binary.LittleEndian.Uint64(a.Read(8))
	logicalImagePathLength := binary.LittleEndian.Uint32(a.Read(4))

	if version == 4 {
		a.Read(44) // ?
	}

	logicalImagePath := string(a.read(int(logicalImagePathLength)))
	fmt.Println("- Logical Image Path:", logicalImagePath)

	if logicalImagePath != "Custom Content Image([Multi])" {
		t, _ := a.f.Seek(0, io.SeekCurrent)
		a.f.Seek(int64(a.margin)+int64(imageHeaderLength2)-t, 1)
	}
	return true
}

func (a *AD1ReaderController) decompressBytes(source []byte) []byte {
	b := bytes.NewReader(source)
	z, err := zlib.NewReader(b)
	if err != nil {
		return []byte{}
	}
	defer z.Close()
	p, err := ioutil.ReadAll(z)
	if err != nil {
		return []byte{}
	}
	return p
}

func (a *AD1ReaderController) readContent(process func(item AD1Item)) {
	//items := map[int]string{}
	t, _ := a.f.Seek(0, io.SeekCurrent)
	footerBytes, _ := hex.DecodeString("4154545247554944")
	lastP := float64(0)
	for t < a.size-int64(a.margin) {
		p := math.Round((float64(t) / (float64(a.size) - float64(a.margin))) * 100.0)
		if p > lastP {
			fmt.Printf("\r%v %%", p)
			lastP = p
		}
		ff := binary.LittleEndian.Uint32(a.Read(4))
		if ff == binary.LittleEndian.Uint32(footerBytes) {
			break // TODO
		}
		binary.LittleEndian.Uint32(a.Read(4))
		binary.LittleEndian.Uint64(a.Read(8))
		nextBlock := binary.LittleEndian.Uint64(a.Read(8))
		startOfData := binary.LittleEndian.Uint64(a.Read(8))
		decompSize := binary.LittleEndian.Uint64(a.Read(8))

		itemType := binary.LittleEndian.Uint32(a.Read(4))
		filenameLength := binary.LittleEndian.Uint32(a.Read(4))

		nextBlock += uint64(a.margin)
		startOfData += uint64(a.margin)

		filename := string(a.Read(int(filenameLength)))
		folderIndex := binary.LittleEndian.Uint64(a.Read(8))

		/*parent_path := items[int(folderIndex)+a.margin]

		path := ""
		if parent_path != "" {
			path = strings.Join([]string{parent_path, filename}, "/")
		} else {
			path = filename
		}*/

		//items[int(t)] = path
		content := []byte{}
		if decompSize > 0 {
			chunkCount := binary.LittleEndian.Uint64(a.Read(8)) + 1
			chunks := []uint64{}
			for i := 0; i < int(chunkCount); i++ {
				chunks = append(chunks, binary.LittleEndian.Uint64(a.Read(8)))
			}
			for i := 1; i < len(chunks); i++ {
				compressed := a.Read(int(chunks[i] - chunks[i-1]))
				content = append(content, a.decompressBytes(compressed)...)
			}
		}

		metadata := map[uint32](map[uint32]string){}

		for nextBlock > 0 {
			nextBlock = binary.LittleEndian.Uint64(a.Read(8))
			category := binary.LittleEndian.Uint32(a.Read(4))
			key := binary.LittleEndian.Uint32(a.Read(4))
			valueLength := binary.LittleEndian.Uint32(a.Read(4))

			if _, ok := metadata[category]; !ok {
				metadata[category] = map[uint32]string{}
			}
			metadata[category][key] = string(a.Read(int(valueLength)))

		}
		parentIdx := 0
		if folderIndex != 0 {
			parentIdx = int(folderIndex) + a.margin
		}
		process(AD1Item{Filename: filename, Parent: parentIdx, Metadata: metadata, Content: content, Type: itemType, Id: t})

		t, _ = a.f.Seek(0, io.SeekCurrent)
	}
	fmt.Printf("\r100 %%")
}

func main() {
	output := flag.String("o", "./output", "Target to extract the image file to")
	source := flag.String("f", "", "AD1 file")
	flag.Parse()
	if _, err := os.Stat(*output); os.IsNotExist(err) {
		fmt.Println("Target path:", *output, "not found")
		return
	}
	var goad1 AD1ReaderController
	goad1.Init(*source, *output)
	directory := AD1Directory{}
	goad1.ReadContent(func(newItem AD1Item) {
		directory.Items = append(directory.Items, newItem)
		item := &directory.Items[len(directory.Items)-1]
		m1 := regexp.MustCompile(`[\\\\/:*?\"<>|]`)
		item.Filename = m1.ReplaceAllString(item.Filename, "_")
		conPath := directory.GetItemPath(item)
		fullpath := strings.Join([]string{*output, conPath}, "/")
		if item.Type == 5 {
			err := os.Mkdir(fullpath, os.ModePerm)
			if err != nil {
				fmt.Println("Failed to create directory:", fullpath) // TODO
				os.Exit(1)
			}
		} else if item.Type == 0 {
			f, err := os.Create(fullpath)
			if err != nil {
				fmt.Println("Failed to create file:", fullpath) // TODO
				os.Exit(1)
			}
			f.Write(item.Content)
			defer f.Close()
		}
	})
	goad1.Close()
}
