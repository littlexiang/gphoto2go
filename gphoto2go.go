package gphoto2go

// #cgo pkg-config: libgphoto2
// #include <gphoto2.h>
// #include <gphoto2-setting.h>
// #include <stdlib.h>
// #include <gphoto2-camera.h>
import "C"
import (
	"io"
	"log"
	"reflect"
	"strings"
	"unsafe"
)

const (
	CAPTURE_IMAGE           = C.GP_CAPTURE_IMAGE
	CAPTURE_MOVIE           = C.GP_CAPTURE_MOVIE
	CAPTURE_SOUND           = C.GP_CAPTURE_SOUND
	GP_WIDGET_MENU          = C.GP_WIDGET_MENU
	GP_WIDGET_RADIO         = C.GP_WIDGET_RADIO
	GP_WIDGET_TEXT          = C.GP_WIDGET_TEXT
	GP_ERROR_BAD_PARAMETERS = C.GP_ERROR_BAD_PARAMETERS
	GP_OK                   = C.GP_OK
)

type Camera struct {
	camera  *C.Camera
	context *C.GPContext
}

var portinfolist *C.GPPortInfoList

type CameraFilePath struct {
	Name   string
	Folder string
}

func (c *Camera) Init(settings [][]string, port string) int {
	c.context = C.gp_context_new()

	for _, cfg := range settings {
		C.gp_setting_set(C.CString(cfg[0]), C.CString(cfg[1]), C.CString(cfg[2]))
	}

	C.gp_camera_new(&c.camera)

	c.initPort(port)

	err := C.gp_camera_init(c.camera, c.context)
	return int(err)
}

func (c *Camera) initPort(port string) int {
	if port != "" {
		ret := C.gp_port_info_list_new(&portinfolist)
		if ret < GP_OK {
			return int(ret)
		}
		ret = C.gp_port_info_list_load(portinfolist)
		if ret < 0 {
			return int(ret)
		}
		ret = C.gp_port_info_list_count(portinfolist)
		if ret < 0 {
			return int(ret)
		}

		var pi C.GPPortInfo
		p := C.gp_port_info_list_lookup_path(portinfolist, C.CString(port))
		ret = C.gp_port_info_list_get_info(portinfolist, p, &pi)
		if ret < GP_OK {
			return int(ret)
		}
		ret = C.gp_camera_set_port_info(c.camera, pi)
		if ret < GP_OK {
			return int(ret)
		}
	}

	return 0
}

func (c *Camera) Exit() int {
	err := C.gp_camera_exit(c.camera, c.context)
	return int(err)
}

func (c *Camera) Cancel() {
	C.gp_context_cancel(c.context)
}

func (c *Camera) GetAbilities() (C.CameraAbilities, int) {
	var abilities C.CameraAbilities
	err := C.gp_camera_get_abilities(c.camera, &abilities)
	return abilities, int(err)
}

func (c *Camera) TriggerCapture() int {
	err := C.gp_camera_trigger_capture(c.camera, c.context)
	return int(err)
}

func (c *Camera) TriggerCaptureToFile() (CameraFilePath, int) {
	var path CameraFilePath
	var _path C.CameraFilePath
	err := C.gp_camera_capture(c.camera, CAPTURE_IMAGE, &_path, c.context)
	path.Name = C.GoString(&_path.name[0])
	path.Folder = C.GoString(&_path.folder[0])
	return path, int(err)
}

type CameraEventType int

const (
	EventUnknown     CameraEventType = C.GP_EVENT_UNKNOWN
	EventTimeout     CameraEventType = C.GP_EVENT_TIMEOUT
	EventFileAdded   CameraEventType = C.GP_EVENT_FILE_ADDED
	EventFileChanged CameraEventType = C.GP_EVENT_FILE_CHANGED
)

type CameraEvent struct {
	Type   CameraEventType
	Folder string
	File   string
}

func (c *Camera) AsyncWaitForEvent(timeout int) chan *CameraEvent {
	var eventType C.CameraEventType
	var vp unsafe.Pointer
	defer C.free(vp)

	ch := make(chan *CameraEvent)

	go func() {
		C.gp_camera_wait_for_event(c.camera, C.int(timeout), &eventType, &vp, c.context)
		ch <- cCameraEventToGoCameraEvent(vp, eventType)
	}()

	return ch
}

func cCameraEventToGoCameraEvent(voidPtr unsafe.Pointer, eventType C.CameraEventType) *CameraEvent {
	ce := new(CameraEvent)
	ce.Type = CameraEventType(eventType)

	if ce.Type == EventFileAdded || ce.Type == EventFileChanged {
		cameraFilePath := (*C.CameraFilePath)(voidPtr)
		ce.File = C.GoString((*C.char)(&cameraFilePath.name[0]))
		ce.Folder = C.GoString((*C.char)(&cameraFilePath.folder[0]))
	}

	return ce
}

func (c *Camera) ListFolders(folder string) ([]string, int) {
	if folder == "" {
		folder = "/"
	}

	var cameraList *C.CameraList
	C.gp_list_new(&cameraList)
	defer C.free(unsafe.Pointer(cameraList))

	cFolder := C.CString(folder)
	defer C.free(unsafe.Pointer(cFolder))

	err := C.gp_camera_folder_list_folders(c.camera, cFolder, cameraList, c.context)
	folderMap, _ := cameraListToMap(cameraList)

	names := make([]string, len(folderMap))
	i := 0
	for key, _ := range folderMap {
		names[i] = key
		i += 1
	}

	return names, int(err)
}

func (c *Camera) RListFolders(folder string) []string {
	folders := make([]string, 0)
	path := folder
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	subfolders, _ := c.ListFolders(path)
	for _, sub := range subfolders {
		subPath := path + sub
		folders = append(folders, subPath)
		folders = append(folders, c.RListFolders(subPath)...)
	}

	return folders
}

func (c *Camera) ListFiles(folder string) ([]string, int) {
	if folder == "" {
		folder = "/"
	}

	if !strings.HasSuffix(folder, "/") {
		folder = folder + "/"
	}

	var cameraList *C.CameraList
	C.gp_list_new(&cameraList)
	defer C.free(unsafe.Pointer(cameraList))

	cFolder := C.CString(folder)
	defer C.free(unsafe.Pointer(cFolder))

	err := C.gp_camera_folder_list_files(c.camera, cFolder, cameraList, c.context)
	fileNameMap, _ := cameraListToMap(cameraList)

	names := make([]string, len(fileNameMap))
	i := 0
	for key, _ := range fileNameMap {
		names[i] = key
		i += 1
	}

	return names, int(err)
}

func cameraListToMap(cameraList *C.CameraList) (map[string]string, int) {
	size := int(C.gp_list_count(cameraList))
	vals := make(map[string]string)

	if size < 0 {
		return vals, size
	}

	for i := 0; i < size; i++ {
		var cKey *C.char
		var cVal *C.char

		C.gp_list_get_name(cameraList, C.int(i), &cKey)
		C.gp_list_get_value(cameraList, C.int(i), &cVal)
		defer C.free(unsafe.Pointer(cKey))
		defer C.free(unsafe.Pointer(cVal))
		key := C.GoString(cKey)
		val := C.GoString(cVal)

		vals[key] = val
	}

	return vals, 0
}

func (c *Camera) Model() (string, int) {
	abilities, err := c.GetAbilities()
	model := C.GoString((*C.char)(&abilities.model[0]))

	return model, err
}

func CameraResultToString(err int) string {
	return C.GoString(C.gp_result_as_string(C.int(err)))
}

// Need to find a good buffer size
// For now, let's try 1MB
const fileReaderBufferSize = 1 * 1024 * 1024

type cameraFileReader struct {
	camera   *Camera
	folder   string
	fileName string
	fullSize uint64
	offset   uint64
	closed   bool

	cCameraFile *C.CameraFile
	cBuffer     *C.char

	buffer [fileReaderBufferSize]byte
}

func (cfr *cameraFileReader) Read(p []byte) (int, error) {
	if cfr.closed {
		return 0, io.ErrClosedPipe
	}

	n := uint64(len(p))

	if n == 0 {
		return 0, nil
	}

	bufLen := uint64(len(cfr.buffer))
	remaining := cfr.fullSize - cfr.offset

	toRead := bufLen
	if toRead > remaining {
		toRead = remaining
	}

	if toRead > n {
		toRead = n
	}

	// From: https://code.google.com/p/go-wiki/wiki/cgo
	// Turning C arrays into Go slices
	sliceHeader := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(cfr.cBuffer)),
		Len:  int(cfr.fullSize),
		Cap:  int(cfr.fullSize),
	}
	goSlice := *(*[]C.char)(unsafe.Pointer(&sliceHeader))

	for i := uint64(0); i < toRead; i++ {
		p[i] = byte(goSlice[cfr.offset+i])
	}

	cfr.offset += toRead

	if cfr.offset < cfr.fullSize {
		return int(toRead), nil
	}
	return int(toRead), io.EOF
}

func (cfr *cameraFileReader) Close() error {
	if !cfr.closed {
		// If I understand correctly, freeing the CameraFile will also free the data buffer (ie. cfr.cBuffer)
		C.gp_file_free(cfr.cCameraFile)
		cfr.closed = true
	}
	return nil
}

func (c *Camera) FileReader(folder string, fileName string) io.ReadCloser {
	cfr := new(cameraFileReader)
	cfr.camera = c
	cfr.folder = folder
	cfr.fileName = fileName
	cfr.offset = 0
	cfr.closed = false

	cFileName := C.CString(cfr.fileName)
	cFolderName := C.CString(cfr.folder)
	defer C.free(unsafe.Pointer(cFileName))
	defer C.free(unsafe.Pointer(cFolderName))

	C.gp_file_new(&cfr.cCameraFile)
	C.gp_camera_file_get(c.camera, cFolderName, cFileName, C.GP_FILE_TYPE_NORMAL, cfr.cCameraFile, c.context)

	var cSize C.ulong
	C.gp_file_get_data_and_size(cfr.cCameraFile, &cfr.cBuffer, &cSize)

	cfr.fullSize = uint64(cSize)

	return cfr
}

func (c *Camera) DeleteFile(folder, file string) int {
	folderBytes := []byte(folder)
	fileBytes := []byte(file)
	//Convert the byte arrays into C pointers

	folderPointer := (*C.char)(unsafe.Pointer(&folderBytes[0]))
	filePointer := (*C.char)(unsafe.Pointer(&fileBytes[0]))
	err := C.gp_camera_file_delete(c.camera, folderPointer, filePointer, c.context)
	return int(err)
}

func _lookup_widget(widget *C.CameraWidget, key string, child **C.CameraWidget) C.int {
	ret := C.gp_widget_get_child_by_name(widget, C.CString(key), child)
	if ret < GP_OK {
		ret = C.gp_widget_get_child_by_label(widget, C.CString(key), child)
	}
	return ret
}

func (c *Camera) SetConfig(key string, val string) int {
	var widget *C.CameraWidget
	var child *C.CameraWidget

	var widgetType C.CameraWidgetType

	ret := C.gp_camera_get_config(c.camera, &widget, c.context)
	if ret < GP_OK {
		log.Printf("camera_get_config failed: %d\n", ret)
		return int(ret)
	}

	ret = _lookup_widget(widget, key, &child)
	defer func() {
		C.gp_widget_free(widget)
	}()

	if ret < GP_OK {
		log.Printf("lookup widget failed: %d\n", ret)
		return int(ret)
	}

	/* This type check is optional, if you know what type the label
	 * has already. If you are not sure, better check. */
	ret = C.gp_widget_get_type(child, &widgetType)
	if ret < GP_OK {
		log.Printf("widget get type failed: %d\n", ret)
		return int(ret)
	}

	switch widgetType {
	case GP_WIDGET_MENU:
	case GP_WIDGET_TEXT:
	case GP_WIDGET_RADIO:
		// ok
	default:
		log.Printf("widget has bad type %d\n", widgetType)
		return int(GP_ERROR_BAD_PARAMETERS)
	}

	cVal := C.CString(val)
	ret = C.gp_widget_set_value(child, unsafe.Pointer(cVal))
	log.Println("config info:", child, widget, widgetType)
	/* This is the actual set call. Note that we keep
	 * ownership of the string and have to free it if necessary.
	 */
	log.Printf("config set: %#v %#v %#v", key, val, ret)
	if ret < GP_OK {
		return int(ret)
	}

	ret = C.gp_camera_set_single_config(c.camera, C.CString(key), child, c.context)
	log.Println("gp_camera_set_single_config:", key, ret)

	if ret != GP_OK {
		/* This stores it on the camera again */
		ret = C.gp_camera_set_config(c.camera, widget, c.context)
		log.Println("gp_camera_set_config:", key, ret)
	}

	return int(ret)
}
