package reader

import (
	"context"
	"encoding/json"
	"fmt"
	vmv1 "github.com/example/virtualmachine/api/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"time"
)

type ObjectKey = types.NamespacedName

type VMReader struct {
	// 클러스터에 연결하기 위한 클라이언트 또는 기타 필요한 필드를 여기에 추가할 수 있습니다.
	client *http.Client
}

func NewVMReader() *VMReader {
	return &VMReader{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Get 메서드 구현
func (r *VMReader) Get(ctx context.Context, name types.NamespacedName, vm *vmv1.VirtualMachine) error {
	//func (r *VMReader) Get(request reconcile.Request, obj *vmv1.VirtualMachine) error {
	url := fmt.Sprintf("http://localhost:8888/virtualmachines/%s", name.Name)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get object, status code: %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(vm)
	if err != nil {
		return err
	}
	return nil
}

// Delete 메서드 구현
func (r *VMReader) DeleteVirtualMachine(vm *vmv1.VirtualMachine) error {
	url := fmt.Sprintf("http://localhost:8888/virtualmachine/%s", vm.Name)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete virtual machine, status code: %d", resp.StatusCode)
	}

	return nil
}
