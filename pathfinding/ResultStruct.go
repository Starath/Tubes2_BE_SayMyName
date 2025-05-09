package pathfinding

type PathStep struct {
	ChildName   string
	Parent1Name string
	Parent2Name string
}

type Result struct {
	Path         []PathStep
	NodesVisited int
}

type MultipleResult struct {
	Results []Result
}
