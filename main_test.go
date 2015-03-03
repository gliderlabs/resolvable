package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func tempdir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("could not create temp dir:", err)
	}
	return dir
}

func assertFileContains(t *testing.T, path, expected string) {
	got, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read '%v': %v", path, err)
	}

	if string(got) != expected {
		t.Errorf("expected file %v to be:\n%v\n\nbut got:\n%v", path, expected, string(got))
	}
}

func checkInsertLine(t *testing.T, path, line, orig string) {
	err := insertLine(line, path)
	if err != nil {
		t.Fatal("could not insert line:", err)
	}

	assertFileContains(t, path, line+"\n"+orig)
}

func TestInsertLineNewFile(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.txt")
	checkInsertLine(t, path, "hello world", "")
}

func TestInsertLineEmptyFile(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.txt")
	err := ioutil.WriteFile(path, []byte{}, 0666)
	if err != nil {
		t.Fatal("could not create file:", err)
	}
	checkInsertLine(t, path, "hello world", "")
}

func TestInsertLineExistingFile(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.txt")
	orig := "existing text\nanother line\n"
	err := ioutil.WriteFile(path, []byte(orig), 0666)
	if err != nil {
		t.Fatal("could not create file:", err)
	}
	checkInsertLine(t, path, "hello world", orig)
}

func checkRemoveLine(t *testing.T, path, line, expected string) {
	err := removeLine(line, path)
	if err != nil {
		t.Fatal("could not insert line:", err)
	}

	assertFileContains(t, path, expected)
}

func TestRemoveLineBeginning(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.txt")
	line := "hello world"
	rest := "some more\ntext after\n"

	err := ioutil.WriteFile(path, []byte(line+"\n"+rest), 0666)
	if err != nil {
		t.Fatal("could not create file:", err)
	}

	checkRemoveLine(t, path, line, rest)
}

func TestRemoveLineMiddle(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.txt")
	line := "hello world"
	pre := "some\nbefore\n"
	post := "more\nafter\n"

	err := ioutil.WriteFile(path, []byte(pre+line+"\n"+post), 0666)
	if err != nil {
		t.Fatal("could not create file:", err)
	}

	checkRemoveLine(t, path, line, pre+post)
}

func TestRemoveLineEnd(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.txt")
	line := "hello world"
	pre := "some\nbefore\n"

	err := ioutil.WriteFile(path, []byte(pre+line), 0666)
	if err != nil {
		t.Fatal("could not create file:", err)
	}

	checkRemoveLine(t, path, line, pre)
}

func TestRemoveLineMulti(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.txt")
	line := "hello world"
	pre := "some\nbefore\n"
	mid := "and\nbetween\n"
	post := "more\nafter\n"

	origText := pre + line + "\n" + mid + line + "\n" + post
	expected := pre + mid + post

	err := ioutil.WriteFile(path, []byte(origText), 0666)
	if err != nil {
		t.Fatal("could not create file:", err)
	}

	checkRemoveLine(t, path, line, expected)
}
