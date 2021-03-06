package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	thorchain "github.com/thorchain/THORChain/app"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

func init() {
	rootCmd.AddCommand(txCmd)
	rootCmd.AddCommand(txServerCmd)
	rootCmd.AddCommand(pubkeyCmd)
	rootCmd.AddCommand(addrCmd)
	rootCmd.AddCommand(hackCmd)
	rootCmd.AddCommand(rawBytesCmd)
}

var rootCmd = &cobra.Command{
	Use:          "thorchaindebug",
	Short:        "THORChain debug tool",
	SilenceUsage: true,
}

var txCmd = &cobra.Command{
	Use:   "tx",
	Short: "Decode a thorchain tx from hex or base64",
	RunE:  runTxCmd,
}

var txServerCmd = &cobra.Command{
	Use:   "tx-decoding-server",
	Short: "Starts a server that listens to a unix socket to decode a thorchain tx from hex or base64",
	RunE:  runTxServerCmd,
}

var pubkeyCmd = &cobra.Command{
	Use:   "pubkey",
	Short: "Decode a pubkey from hex, base64, or bech32",
	RunE:  runPubKeyCmd,
}

var addrCmd = &cobra.Command{
	Use:   "addr",
	Short: "Convert an address between hex and bech32",
	RunE:  runAddrCmd,
}

var hackCmd = &cobra.Command{
	Use:   "hack",
	Short: "Boilerplate to Hack on an existing state by scripting some Go...",
	RunE:  runHackCmd,
}

var rawBytesCmd = &cobra.Command{
	Use:   "raw-bytes",
	Short: "Convert raw bytes output (eg. [10 21 13 255]) to hex",
	RunE:  runRawBytesCmd,
}

func runRawBytesCmd(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected single arg")
	}
	stringBytes := args[0]
	stringBytes = strings.Trim(stringBytes, "[")
	stringBytes = strings.Trim(stringBytes, "]")
	spl := strings.Split(stringBytes, " ")

	byteArray := []byte{}
	for _, s := range spl {
		b, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		byteArray = append(byteArray, byte(b))
	}
	fmt.Printf("%X\n", byteArray)
	return nil
}

func runPubKeyCmd(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected single arg")
	}

	pubkeyString := args[0]
	var pubKeyI crypto.PubKey

	// try hex, then base64, then bech32
	pubkeyBytes, err := hex.DecodeString(pubkeyString)
	if err != nil {
		var err2 error
		pubkeyBytes, err2 = base64.StdEncoding.DecodeString(pubkeyString)
		if err2 != nil {
			var err3 error
			pubKeyI, err3 = sdk.GetAccPubKeyBech32(pubkeyString)
			if err3 != nil {
				var err4 error
				pubKeyI, err4 = sdk.GetValPubKeyBech32(pubkeyString)

				if err4 != nil {
					return fmt.Errorf(`Expected hex, base64, or bech32. Got errors:
			hex: %v,
			base64: %v
			bech32 acc: %v
			bech32 val: %v
			`, err, err2, err3, err4)

				}
			}

		}
	}

	var pubKey ed25519.PubKeyEd25519
	if pubKeyI == nil {
		copy(pubKey[:], pubkeyBytes)
	} else {
		pubKey = pubKeyI.(ed25519.PubKeyEd25519)
		pubkeyBytes = pubKey[:]
	}

	cdc := thorchain.MakeCodec()
	pubKeyJSONBytes, err := cdc.MarshalJSON(pubKey)
	if err != nil {
		return err
	}
	accPub, err := sdk.Bech32ifyAccPub(pubKey)
	if err != nil {
		return err
	}
	valPub, err := sdk.Bech32ifyValPub(pubKey)
	if err != nil {
		return err
	}
	fmt.Println("Address:", pubKey.Address())
	fmt.Printf("Hex: %X\n", pubkeyBytes)
	fmt.Println("JSON (base64):", string(pubKeyJSONBytes))
	fmt.Println("Bech32 Acc:", accPub)
	fmt.Println("Bech32 Val:", valPub)
	return nil
}

func runAddrCmd(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected single arg")
	}

	addrString := args[0]
	var addr []byte

	// try hex, then bech32
	var err error
	addr, err = hex.DecodeString(addrString)
	if err != nil {
		var err2 error
		addr, err2 = sdk.AccAddressFromBech32(addrString)
		if err2 != nil {
			var err3 error
			addr, err3 = sdk.ValAddressFromBech32(addrString)

			if err3 != nil {
				return fmt.Errorf(`Expected hex or bech32. Got errors:
			hex: %v,
			bech32 acc: %v
			bech32 val: %v
			`, err, err2, err3)

			}
		}
	}

	accAddr := sdk.AccAddress(addr)
	valAddr := sdk.ValAddress(addr)

	fmt.Println("Address:", addr)
	fmt.Println("Bech32 Acc:", accAddr)
	fmt.Println("Bech32 Val:", valAddr)
	return nil
}

func runTxCmd(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected single arg")
	}

	txString := args[0]

	// try hex, then base64
	txBytes, err := hex.DecodeString(txString)
	if err != nil {
		var err2 error
		txBytes, err2 = base64.StdEncoding.DecodeString(txString)
		if err2 != nil {
			return fmt.Errorf(`Expected hex or base64. Got errors:
			hex: %v,
			base64: %v
			`, err, err2)
		}
	}

	var tx = auth.StdTx{}
	cdc := thorchain.MakeCodec()

	err = cdc.UnmarshalBinary(txBytes, &tx)
	if err != nil {
		return err
	}

	bz, err := cdc.MarshalJSON(tx)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer([]byte{})
	err = json.Indent(buf, bz, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(buf.String())
	return nil
}

func runTxServerCmd(_ *cobra.Command, _ []string) error {
	os.Remove("/tmp/thorchaindebug-tx-decoding.sock")
	l, err := net.Listen("unix", "/tmp/thorchaindebug-tx-decoding.sock")

	if err != nil {
		return fmt.Errorf("listen error %v", err)
	}

	defer l.Close()

	cdc := thorchain.MakeCodec()

	for {
		c, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accept error %v", err)
		}

		go txServer(c, cdc)
	}
}

func txServer(c net.Conn, cdc *wire.Codec) {
	for {
		buf := make([]byte, 10240)
		nr, err := c.Read(buf)
		if err != nil {
			if err == io.EOF {
				// connection closed => just return the server
				// fmt.Println("Connection closed")
				return
			}
			fmt.Println("Could not read:", err)
			continue
		}

		txString := string(buf[0:nr])

		// fmt.Println("Server got tx:", txString)

		// try hex, then base64
		txBytes, err := hex.DecodeString(txString)
		if err != nil {
			var err2 error
			txBytes, err2 = base64.StdEncoding.DecodeString(txString)
			if err2 != nil {
				fmt.Printf(`Expected hex or base64. Got errors:
				hex: %v,
				base64: %v
				`, err, err2)
				continue
			}
		}

		var tx = auth.StdTx{}

		err = cdc.UnmarshalBinary(txBytes, &tx)
		if err != nil {
			fmt.Println("Unmarshal binary error:", err)
			continue
		}

		bz, err := cdc.MarshalJSON(tx)
		if err != nil {
			fmt.Println("Marshal json error:", err)
			continue
		}

		buff := bytes.NewBuffer([]byte{})
		err = json.Indent(buff, bz, "", "  ")
		if err != nil {
			fmt.Println("Json indent error:", err)
			continue
		}

		// fmt.Println(buff.String())
		// continue

		_, err = c.Write(buff.Bytes())
		if err != nil {
			fmt.Println("Write error:", err)
		}
	}
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
