package cmd

import (
	"strings"

	cio "github.com/openbiox/butils/io"
	cnet "github.com/openbiox/butils/net"
	"github.com/openbiox/butils/stringo"
	"github.com/spf13/cobra"
)

var seqCmd = &cobra.Command{
	Use:   "seq [id1 id2 id3... | manifest1 manifest2 manifest3...]",
	Short: "Can be used to access sequence data via unique id (dbGAP and EGA) or manifest files (TCGA).",
	Long:  `Can be used to access sequence data via unique id or manifest files. More see here https://github.com/Miachol/bget.`,
	Run: func(cmd *cobra.Command, args []string) {
		seqCmdRunOptions(cmd)
	},
}

func parseSeq() (seqs map[string][]string) {
	seqs = make(map[string][]string)
	seqsTmp := []string{}
	if bgetClis.seqs != "" && strings.Contains(bgetClis.seqs, bgetClis.separator) {
		seqsTmp = strings.Split(bgetClis.seqs, bgetClis.separator)
	} else if bgetClis.seqs != "" {
		seqsTmp = []string{bgetClis.seqs}
	} else if bgetClis.listFile != "" {
		seqsTmp = cio.ReadLines(bgetClis.listFile)
	}
	for i := range seqsTmp {
		if stringo.StrDetect(strings.ToUpper(seqsTmp[i]), "^GSE|^GPL|^GDS") {
			seqs["geo"] = append(seqs["geo"], seqsTmp[i])
		} else if stringo.StrDetect(strings.ToUpper(seqsTmp[i]), "^SRR|^ERR") {
			seqs["sra"] = append(seqs["sra"], seqsTmp[i])
		} else if stringo.StrDetect(strings.ToLower(seqsTmp[i]), ".krt$") {
			seqs["sraKrt"] = append(seqs["sraKrt"], seqsTmp[i])
		} else if stringo.StrDetect(strings.ToUpper(seqsTmp[i]), "^EGA") {
			seqs["ega"] = append(seqs["ega"], seqsTmp[i])
		} else if stringo.StrDetect(strings.ToLower(seqsTmp[i]), ".txt$") {
			seqs["tcgaManifest"] = append(seqs["tcgaManifest"], seqsTmp[i])
		} else {
			seqs["tcgaFileID"] = append(seqs["tcgaFileID"], seqsTmp[i])
		}
	}
	return seqs
}

func downloadSeq() {
	seqs := parseSeq()
	done := make(map[string][]string)
	sem := make(chan bool, bgetClis.thread)
	netOpt := setNetParams(&bgetClis)
	for k, v := range seqs {
		for i := range v {
			sem <- true
			done[k] = append(done[k], seqs[k][i])
			go func(k string, i int) {
				defer func() {
					<-sem
				}()
				if k == "sra" {
					cnet.Prefetch(seqs[k][i], "", bgetClis.downloadDir, netOpt)
				} else if k == "sraKrt" {
					cnet.Prefetch("", seqs[k][i], bgetClis.downloadDir, netOpt)
				} else if k == "tcgaManifest" {
					cnet.GdcClient("", seqs[k][i], bgetClis.downloadDir, netOpt)
				} else if k == "tcgaFileID" {
					cnet.GdcClient(seqs[k][i], "", bgetClis.downloadDir, netOpt)
				}
			}(k, i)
		}
	}
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}
	sem = make(chan bool, bgetClis.thread)
	for k, v := range seqs {
		for i := range v {
			sem <- true
			done[k] = append(done[k], seqs[k][i])
			go func(k string, i int) {
				defer func() {
					<-sem
				}()
				if k == "geo" {
					Geofetch(seqs[k][i], bgetClis.downloadDir, netOpt)
				}
			}(k, i)
		}
	}
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}
}

func seqCmdRunOptions(cmd *cobra.Command) {
	items := []string{}
	if len(cmd.Flags().Args()) >= 1 {
		items = append(items, cmd.Flags().Args()...)
		bgetClis.seqs = strings.Join(items, bgetClis.separator)
	}

	checkDownloadDir(bgetClis.seqs != "")
	if bgetClis.seqs != "" || bgetClis.listFile != "" {
		downloadSeq()
		bgetClis.helpFlags = false
	}
	if bgetClis.helpFlags {
		cmd.Help()
	}
}

func init() {
	seqCmd.Flags().StringVarP(&(bgetClis.engine), "engine", "g", "go-http", "Point the download engine: go-http, wget, curl, and axel.")
	seqCmd.Flags().BoolVarP(&(bgetClis.uncompress), "uncompress", "u", false, "Uncompress download files for .zip, .tar.gz, and .gz suffix files (now support GEO database).")
	seqCmd.Flags().StringVarP(&(bgetClis.gdcToken), "gdc-token", "", "", "Token to access TCGA portal files.")
	seqCmd.Flags().StringVarP(&(bgetClis.listFile), "list-file", "l", "", "A file contains seq id (e.g. SRR) or manifest files for download.")
	seqCmd.Example = `  bget seq ERR3324530 SRR544879 # download files from SRA databaes
  bget seq GSE23543 # download files from GEO databaes (auto download SRA acc list and run info)
  bget dbgap.krt # download files from dbGap database using krt files
  
  # download TCGA files using file id
  bget seq b7670817-9d6b-494e-9e22-8494e2fd430d

  # download TCGA files using manifest files
  # split for parallel
  split -a 3 --additional-suffix=.txt -l 100 gdc_manifest.2019-08-23-TCGA.txt -d
  for i in x*.txt
  do
    head -n 1 x000.txt > ${i}.tmp
    cat ${i} >> ${i}.tmp
    mv ${i}.tmp ${i}
  done
  sed -i '1d' x000.txt
  bget seq *.txt -t 5

  # support auto (if you do not have *.krt, TCGA manifest, please not include it for test)
  bget seq SRR544879 GSE23543 b7670817-9d6b-494e-9e22-8494e2fd430d dbgap.krt *.txt -t 5`
}