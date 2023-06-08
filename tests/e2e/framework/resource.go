package framework

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	psapi "k8s.io/pod-security-admission/api"
)

const maxExecutiveEmpties = 100

// Scanner scans a yaml manifest file for manifest tokens delimited by "---".
// See bufio.Scanner for semantics.
type Scanner struct {
	reader  *k8syaml.YAMLReader
	token   []byte // Last token returned by split.
	err     error  // Sticky error.
	empties int    // Count of successive empty tokens.
	done    bool   // Scan has finished.
}

func NewYAMLScanner(r io.Reader) *Scanner {
	return &Scanner{reader: k8syaml.NewYAMLReader(bufio.NewReader(r))}
}

func (s *Scanner) Err() error {
	if s.err == io.EOF {
		return nil
	}
	return s.err
}

func (s *Scanner) Scan() bool {
	if s.done {
		return false
	}

	var (
		tok []byte
		err error
	)

	for {
		tok, err = s.reader.Read()
		if err != nil {
			if err == io.EOF {
				s.done = true
			}
			s.err = err
			return false
		}
		if len(bytes.TrimSpace(tok)) == 0 {
			s.empties++
			if s.empties > maxExecutiveEmpties {
				panic("yaml.Scan: too many empty tokens without progressing")
			}
			continue
		}
		s.empties = 0
		s.token = tok
		return true
	}
}

func (s *Scanner) Text() string {
	return string(s.token)
}

func (s *Scanner) Bytes() []byte {
	return s.token
}

// GetOperatorNamespace will return an Operator Namespace,
// if the flag --operator-namespace  not be used (TestOpeatorNamespaceEnv not set)
// then it will create a new namespace with randon name and return that namespace
func (ctx *Context) GetOperatorNamespace() (string, error) {
	var err error
	ctx.operatorNamespace, err = ctx.getNamespace(ctx.operatorNamespace)
	return ctx.operatorNamespace, err
}

func (ctx *Context) getNamespace(ns string) (string, error) {
	// create namespace only if it doesn't already exist
	_, err := ctx.kubeclient.CoreV1().Namespaces().Get(context.TODO(), ns, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		namespaceObj := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					psapi.EnforceLevelLabel:                          string(psapi.LevelPrivileged),
					"security.openshift.io/scc.podSecurityLabelSync": "false",
				},
			},
		}

		log.Printf("creating namespace %s", ns)
		_, err = ctx.kubeclient.CoreV1().Namespaces().Create(context.TODO(), namespaceObj, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			return "", fmt.Errorf("namespace %s already exists: %w", ns, err)
		} else if err != nil {
			return "", err
		}
		return ns, nil
	} else if apierrors.IsAlreadyExists(err) {
		log.Printf("using existing namespace %s", ns)
		return ns, nil
	} else {
		return ns, nil
	}

}

// GetWatchNamespace will return the  namespaces to operator
// watch for changes, if the flag --watch-namespaced not be used
// then it will  return the Operator Namespace.
func (ctx *Context) GetWatchNamespace() (string, error) {
	// if ctx.watchNamespace is already set and not "";
	// then return ctx.watchnamespace
	if ctx.watchNamespace != "" {
		return ctx.watchNamespace, nil
	}
	// if ctx.watchNamespace == "";
	// ensure it was set explicitly using TestWatchNamespaceEnv
	if ns, ok := os.LookupEnv(TestWatchNamespaceEnv); ok {
		return ns, nil
	}
	// get ctx.operatorNamespace (use ctx.GetOperatorNamespace()
	// to make sure ctx.operatorNamespace is not "")
	operatorNamespace, err := ctx.GetOperatorNamespace()
	if err != nil {
		return "", nil
	}
	ctx.watchNamespace = operatorNamespace
	return ctx.watchNamespace, nil
}
