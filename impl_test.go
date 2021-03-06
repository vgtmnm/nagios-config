/*
   Copyright 2017 Odd Eivind Ebbesen

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package nagioscfg

import (
	"container/list"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"testing"
	"strings"
)

var co = NewCfgObj(T_SERVICE)
var keys = [...]string{
	"max_check_attempts",
	"active_checks_enabled",
	"retain_nonstatus_information",
	"service_description",
	"check_command",
	"contact_groups",
}
var cmd = []string{
	"vgt_check_graphite_v2",
	"192.168.0.1",
	"some.prod.solution.queues.queue.DLQ",
	"4000",
	"5000",
	"gt",
}
var cgs = []string{
	"devs",
	"ops",
	"support",
	"managers",
}

var comment = []byte("					    #    lkdsglknag  \n")
var notcomment = []byte("			 define gris")
var blankline = []byte("						    \n")

func TestString(t *testing.T) {
	str := co.Type.String()
	exp := "service"
	if str != exp {
		t.Errorf("Expected String() to return %q, but got %q", exp, str)
	}
}

func TestSet(t *testing.T) {
	overwritten := co.Set(keys[0], "gaupe")
	if overwritten {
		t.Errorf("%q should not exist yet", keys[0])
	}
	ow2 := co.Set(keys[0], "11")
	if !ow2 {
		t.Errorf("Key %q should have been overwritten", keys[0])
	}
}

func TestAdd(t *testing.T) {
	ok := co.Add(keys[1], "1")
	if !ok {
		t.Error("Failed to add second key")
	}
	ok = co.Add(keys[1], "gris")
	if ok {
		t.Errorf("Should not be allowed to add same key %q more than once", keys[1])
	}
}

func TestGet(t *testing.T) {
	ret, exists := co.Get(keys[0])
	if !exists {
		t.Error("Get returned false")
	}
	if ret != "11" { // set in TestSet()
		t.Errorf("Expected %q, but got %q", "11", ret)
	}
}

func TestDel(t *testing.T) {
	k := "dkey"
	v := "dval"
	deleted := co.Del(k)
	if deleted {
		t.Error("Delete non-existing key should return false")
	}
	co.Add(k, v)
	deleted = co.Del(k)
	if !deleted {
		t.Errorf("Failed to delete key %q", k)
	}
	ret, exists := co.Get(k)
	if exists {
		t.Errorf("Key %q should be deleted, but got value %q", k, ret)
	}
}

func TestLongestKey(t *testing.T) {
	lk := co.LongestKey()
	correct_len := len(keys[1])
	if lk != correct_len {
		t.Errorf("LongestKey() returned %d when correct length is %d", lk, correct_len)
	}
}

func TestSetList(t *testing.T) {
	exists := co.SetList(keys[4], SEP_CMD, cmd...)
	if exists {
		t.Errorf("key %q should not exist yet", keys[4])
	}
}

func TestAddList(t *testing.T) {
	ok := co.AddList(keys[5], SEP_LST, cgs...)
	if !ok {
		t.Error("Failed to add contact groups")
	}
	ok = co.AddList(keys[5], SEP_LST, "gris", "hund", "katt")
	if ok {
		t.Errorf("Should not be allowed to add list to key %q more than once", keys[5])
	}
}

func TestGetList(t *testing.T) {
	lst := co.GetList(keys[5], SEP_LST)
	if lst == nil {
		t.Errorf("Should get a valid list from key %q", keys[5])
	}
	if !reflect.DeepEqual(lst, cgs) {
		t.Error("Returned list is not equal to the one we put in")
	}
}

func TestGetCheckCommand(t *testing.T) {
	lst := co.GetCheckCommand()
	if lst == nil {
		t.Error("Check command list should not be nil")
	}
	if !reflect.DeepEqual(lst, cmd) {
		t.Error("Returned command list does not equal what we put in")
	}
}

func TestGetCheckCommandCmd(t *testing.T) {
	checkcmd, ok := co.GetCheckCommandCmd()
	if !ok {
		t.Errorf("GetCheckCommandCmd() failed to return %q", cmd[0])
	}
	if checkcmd != cmd[0] {
		t.Errorf("Command %q is not equal to %q", checkcmd, cmd[0])
	}
}

func TestGetCheckCommandArgs(t *testing.T) {
	args := co.GetCheckCommandArgs()
	if args == nil {
		t.Error("GetCheckCommandArgs() returned nil")
	}
}

func TestGetName(t *testing.T) {
	o := NewCfgObj(T_COMMAND)
	key := "command_name"
	name := "testcommand"
	o.Set(key, name)
	ret, exists := o.GetName()
	if !exists {
		t.Errorf("Expected %q, but got nothing", name)
	}
	if ret != name {
		t.Errorf("Expected %q, but got %q", name, ret)
	}

	o = NewCfgObj(T_HOST)
	key = "name"
	name = "host-template-something"
	o.Set(key, name)
	ret, exists = o.GetName()
	if !exists {
		t.Errorf("Expected %q, but got nothing", name)
	}
	if ret != name {
		t.Errorf("Expected %q, but got %q", name, ret)
	}
}

func TestGetHostname(t *testing.T) {
	o := NewCfgObj(T_HOST)
	k := "host_name"
	v := "printserver"
	o.Set(k, v)
	ret, exists := o.GetHostname()
	if !exists {
		t.Errorf("Expected %q, but got nothing", v)
		//o.Print(os.Stdout)
	}
	if ret != v {
		t.Errorf("Expected %q, but got %q", v, ret)
	}
}

func TestGetDescription(t *testing.T) {
	o := NewCfgObj(T_SERVICE)
	key := "service_description"
	name := "testservice"
	o.Set(key, name)
	ret, exists := o.GetDescription()
	if !exists {
		t.Errorf("Expected %q, but got nothing", name)
	}
	if ret != name {
		t.Errorf("Expected %q, but got %q", name, ret)
	}
}

func TestGetUniqueCheckName(t *testing.T) {
	o := NewCfgObj(T_SERVICE)
	k1 := "host_name"
	k2 := "service_description"
	v1 := "host.domain.tld"
	v2 := "PLING_PLONG_LuftBallong"
	exp := fmt.Sprintf("%s;%s", v1, v2)
	o.Set(k1, v1)
	o.Set(k2, v2)
	ret, ok := o.GetUniqueCheckName()
	if !ok {
		t.Errorf("Expected %q but got nothing", exp)
	}
	if exp == "" {
		t.Errorf("Expected %q but got %q", exp, ret)
	}
	t.Logf("Unique name: %q", ret)
}

func TestGenerateComment(t *testing.T) {
	co.Add(keys[3], "Graphite DLQ")
	ok := co.generateComment()
	if !ok {
		t.Error("Attempt to generate comment returned false")
	}
	exp_comment := "# service 'Graphite DLQ'"
	if co.Comment != exp_comment {
		t.Errorf("Expected comment %q, but got %q", exp_comment, co.Comment)
	}

}

func TestPrintPropsSorted(t *testing.T) {
	objstr := `#comment 
define service{
	service_description PigInABlanket
	host_name pighost04
	contacts odd
	}`
	str_r := strings.NewReader(objstr)
	rdr := NewReader(str_r)
	co, err := rdr.Read(false, "/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	if co == nil {
		t.Fatal("CfgObj is nil")
	}
	co.PrintPropsSorted(os.Stdout, "%s = %s\n")
}

func BenchmarkPrintProps(b *testing.B) {
	objstr := `#comment 
define service{
	service_description PigInABlanket
	host_name pighost04
	}`
	str_r := strings.NewReader(objstr)
	rdr := NewReader(str_r)
	co, err := rdr.Read(false, "/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	if co == nil {
		b.Fatal("CfgObj is nil")
	}
	fstr := "%-32s %s\n"
	for i := 0; i <= b.N; i++ {
		co.PrintProps(ioutil.Discard, fstr)
	}
}

func BenchmarkPrintPropsSorted(b *testing.B) {
	objstr := `#comment 
define service{
	service_description PigInABlanket
	host_name pighost04
	}`
	str_r := strings.NewReader(objstr)
	rdr := NewReader(str_r)
	co, err := rdr.Read(false, "/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	if co == nil {
		b.Fatal("CfgObj is nil")
	}
	fstr := "%-32s %s\n"
	for i := 0; i <= b.N; i++ {
		co.PrintPropsSorted(ioutil.Discard, fstr)
	}
}

func TestPrint(t *testing.T) {
	co.Align = co.LongestKey() + 2
	co.Print(os.Stdout, true)
}

func TestAdd2(t *testing.T) {
	k1 := "host_name"
	k2 := "service_description"
	o := make(CfgObjs, 0, 3)

	o.Add(NewCfgObj(T_SERVICE))
	if len(o) != 1 {
		t.Error("Length should be 1")
	}
	o[0].Add(k1, "host1")
	o[0].Add(k2, "service1")

	o.Add(NewCfgObj(T_SERVICE))
	if len(o) != 2 {
		t.Error("Length should be 2")
	}
	o[1].Add(k1, "host2")
	o[1].Add(k2, "service2")

	o.Add(NewCfgObj(T_SERVICE))
	if len(o) != 3 {
		t.Error("Length should be 3")
	}
	o[2].Add(k1, "host3")
	o[2].Add(k2, "service3")

	o.Print(os.Stdout, true)
}

func TestDel2(t *testing.T) {
	k1 := "host_name"
	k2 := "service_description"
	o := make(CfgObjs, 0, 3)
	for i := 0; i <= 2; i++ {
		o.Add(NewCfgObj(T_SERVICE))
		o[i].Add(k1, fmt.Sprintf("host_%d", i))
		o[i].Add(k2, fmt.Sprintf("service_%d", i))
	}
	if len(o) != 3 {
		t.Error("Length should be 3")
	}
	o.Print(os.Stdout, true)

	t.Log("Deleting element #1")
	o.Del(1)
	if len(o) != 2 {
		t.Error("Length should be 2")
	}
	o.Print(os.Stdout, true)
}

func BenchmarkDel2(b *testing.B) {
	o := make(CfgObjs, b.N, b.N)
	for i := 0; i < b.N; i++ {
		o[i] = &CfgObj{}
		//o.Add(NewCfgObj(T_SERVICE))
		//o[i].Add("host_name", string(i))
		//o[i].Add("service_description", string(i))
	}
	// Testing shows that deleting from the end of the slice is very fast, while deleting from the middle or beginning is horrendously slow.
	// I should try to test the swap-last-and-shrink technique as well
	for i := 0; i < b.N; i++ {
		//o.Del(len(o)-1)
		o.Del(0)
	}
}

func BenchmarkDelFromList(b *testing.B) {
	l := list.New()
	for i := 0; i <= b.N; i++ {
		//l.PushBack(&CfgObj{})
		l.PushBack(NewCfgObj(T_SERVICE))
	}
	for e := l.Front(); e != nil; e = e.Next() {
		l.Remove(e)
	}
}

func BenchmarkNewCfgObj(b *testing.B) {
	for i := 0; i <= b.N; i++ {
		NewCfgObj(T_SERVICE)
	}
}

func BenchmarkNewCfgObjWithUUID(b *testing.B) {
	for i := 0; i <= b.N; i++ {
		NewCfgObjWithUUID(T_SERVICE)
	}
}

func TestMatchAny(t *testing.T) {
	k1 := "host_name"
	k2 := "service_description"
	rx := regexp.MustCompile(`host[0-9]`)
	o := NewCfgObj(T_SERVICE)
	o.Add(k2, "MatchingService")
	o.Add(k1, "host5")

	if !o.MatchAny(rx) {
		t.Error("Should find match, but did not")
	}

	o.Set(k1, "hostfive")
	if o.MatchAny(rx) {
		t.Error("Should not match, but it did")
	}

	o.Add("bogus_key", "somehost666name")
	if !o.MatchAny(rx) {
		t.Error("Should find match, but did not")
	}

	o.Print(os.Stdout, true)
}

func TestMatchKeys(t *testing.T) {
	k1 := "host_name"
	k2 := "service_description"
	k3 := "active_checks_enabled"

	objs := make(CfgObjs, 0, 3)
	objs.Add(NewCfgObj(T_SERVICE))
	objs[0].Add(k1, "DummyHost")
	objs[0].Add(k2, "DummyCheck")
	objs[0].Add(k3, "0")

	objs.Add(NewCfgObj(T_HOST))
	objs[1].Add(k1, "DummyHost2")
	objs[1].Add(k3, "1")

	objs.Add(NewCfgObj(T_SERVICE))
	objs[2].Add(k1, "otherhost")
	objs[2].Add(k2, "OtherCheck")
	objs[2].Add(k3, "1")

	rx := regexp.MustCompile(`Dummy.*`)

	objs.Print(os.Stdout, true)

	if !objs[0].MatchAllKeys(rx, k1, k2) {
		t.Error("Should match, but did not")
	}
	if objs[1].MatchAllKeys(rx, k1, k2) {
		t.Error("Should not match, but did")
	}
	if objs[2].MatchAllKeys(rx, k1, k2) {
		t.Error("Should not match, but did")
	}

	rx = regexp.MustCompile(`[01]`)
	for i := range objs {
		if !objs[i].MatchAllKeys(rx, k3) {
			t.Error("Should match, but did not")
		}
	}
}

// Trying to reach a similarity to nagios-grep here
func TestCfMapMatchKeys(t *testing.T) {
	path := "../op5_automation/cfg/etc/services.cfg"
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rdr := NewReader(file)
	m, err := rdr.ReadAllMap(path)
	if err != nil {
		t.Fatal(err)
	}

	// now we have the whole file, let's search a bit
	rx := regexp.MustCompile(`db_dummy_gso`)
	u := m.MatchAllKeys(rx, "host_name")
	if u == nil {
		t.Fatal("Unable to find any match")
	}

	for i := range u {
		fmt.Printf("=== Matching UUID: %s ===\n", u[i])
		m[u[i]].Print(os.Stdout, true)
	}
}

func TestCfgMapSearch(t *testing.T) {
	path := "../op5_automation/cfg/etc/services.cfg"
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rdr := NewReader(file)
	m, err := rdr.ReadAllMap(path)
	if err != nil {
		t.Fatal(err)
	}

	// now we have the whole file, let's search a bit
	q := NewCfgQuery()
	q.AddKeyRX(`host_name`, `db_dummy.*`)
	q.AddKeyRX(`check_command`, `vgt_oracle_mutex.*`)

	u := m.Search(q)
	if u == nil {
		t.Fatal("No matches")
	}

	for i := range u {
		fmt.Printf("### Matching UUID %q ###\n", u[i])
		m[u[i]].Print(os.Stdout, true)
	}
}
