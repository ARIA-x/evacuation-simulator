package aria_utility_mqtt

// AttendEntity aria/attend/+のエンティティ(全ての開始前、Universe <- Person)
type AttendEntity struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// RegisteredEntity aria/registered/+/+のエンティティ(全ての開始前、Universe -> Person)
type RegisteredEntity struct {
	ID   string `json:"id"`
	From int    `json:"from"`
	To   int    `json:"to"`
}

// TODO : アナウンス方法を変更する
// CycleEntity aria/cycle/+のエンティティ(サイクルの開始、Universe -> Person)
type CycleEntity struct {
	AnnounceStep int `json:"a"`
}

// PreparedEntity aria/prepared/+のエンティティ(サイクルの準備完了、Universe <- Person)
type PreparedEntity struct {
	ID      string      `json:"id"`
	Persons []AllEntity `json:"persons"`
}

// StepEntity aria/persons/+のエンティティ(ステップの完了、Universe <- Person)
type StepEntity struct {
	ID      string      `json:"id"`
	Persons []AllEntity `json:"persons"`
}

// MessageEntity aria/message/+のエンティティ(メッセージ)
type MessageEntity struct {
	Persons []MessageIDEntity   `json:"persons"`
	Nodes   []MessageIDEntity   `json:"nodes"`
	Areas   []MessageAreaEntity `json:"areas"`
}

// MessageIDEntity aria/message/+のエンティティ(子)
type MessageIDEntity struct {
	ID int `json:"id"`
}

// MessageAreaEntity aria/message/+のエンティティ(子)
type MessageAreaEntity struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Size float64 `json:"size"`
}

// CountEntity (1) flood/countのエンティティ(ステップの開始、Universe -> Person)
type CountEntity struct {
	Count int `json:"count"`
}

// AllEntity (2) person/send/allのエンティティ
type AllEntity struct {
	Count      int     `json:"Simulationtime"`
	ID         int     `json:"id"`
	X          float64 `json:"X"`
	Y          float64 `json:"Y"`
	Status     int     `json:"status"`
	InfoAccess int     `json:"infoAccess"`
}

// RouteEntity (3) person/send/start2target/+のエンティティ
type RouteEntity struct {
	StartNID  int `json:"startNID"`
	TargetNID int `json:"targetNID"`
}

// StatusEntity (4) stat/sendのエンティティ
type StatusEntity struct {
	AffectedPerson  int     `json:"AffectedPerson"`
	EvacuatedPerson int     `json:"EvacuatedPerson"`
	MaxFlood        float64 `json:"MaxFlood"`
	TotalFlood      float64 `json:"TotalFlood"`
}

// (5) person/recv/start2target/+は文字の配列

// CameraEntity (6) camera/flood/+および(7) camera/antenna/+のエンティティ
type CameraEntity struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Left   int    `json:"lt_x"`
	Top    int    `json:"lt_y"`
	Right  int    `json:"rb_x"`
	Bottom int    `json:"rb_y"`
	Topic  string `json:"topic"`
	Data   string `json:"data"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
}

// MediaEntity aria/media/+/+のエンティティ(伝達メディア)
type MediaEntity struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Size        float64 `json:"size"`
	Acquisition float64 `json:"acquisition"`
	Type        string  `json:"type"`
}
