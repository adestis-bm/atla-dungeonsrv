package dungeongen

import (
	"errors"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/atla/dungeonsrv/pkg/util"
)

// RandomRoomStrategy...
type RandomRoomStrategy struct {
	Density               RoomDensity
	MaxRooms              int
	UseRandomSeed         bool
	Seed                  int
	MaxRoomWidth          int
	MaxRoomHeight         int
	MinRoomWidth          int
	MinRoomHeight         int
	SpaceBetweenRooms     int
	ChanceOfAdjacentRooms int
	ChanceForDivergence   int
	RoomConnectedness     int
}

type RoomDensity int8

const (
	RoomDensityLow    RoomDensity = 1
	RoomDensityMedium             = 2
	RoomDensityHigh               = 3
	RoomDensityMax                = 4
)

// NewRandomRoomStrategy returns a default RandomRoomStrategy
func NewRandomRoomStrategy() *RandomRoomStrategy {
	return &RandomRoomStrategy{
		Density:               RoomDensityMedium,
		MaxRooms:              -1,
		UseRandomSeed:         true,
		MinRoomWidth:          5,
		MinRoomHeight:         5,
		MaxRoomWidth:          50,
		MaxRoomHeight:         50,
		SpaceBetweenRooms:     2,
		ChanceOfAdjacentRooms: 30,
		ChanceForDivergence:   10,
		RoomConnectedness:     2,
	}
}

func getMaxRoomsForDensity(data *DungeonData, density RoomDensity) int {
	baseFor100x100 := float32(int(density) * 100.0)
	factor := float32(((data.Width + data.Height) / 2) / 100.0)
	return int(baseFor100x100 * factor)
}

func (strategy *RandomRoomStrategy) buildAdjacentRoom(data *DungeonData, newRoom *RoomData) {
	adjacentRoom, direction := strategy.createDuplicateRoom(data, newRoom)

	if !strategy.roomCollidesWithExisting(data, adjacentRoom.Extrude(-1)) {
		addRoomToDungeon(data, adjacentRoom)

		var door Vec2D
		// check at which direction the adjacent room was created and attach a door
		// TODO: create an open room?
		switch direction {
		case DirectionWest:
			door = NewVec2D(newRoom.X, newRoom.Y+2+(rand.Int()%(newRoom.Height-4)))
			break
		case DirectionNorth:
			door = NewVec2D(newRoom.X+2+(rand.Int()%(newRoom.Width-4)), newRoom.Y)
			break
		case DirectionEast:
			door = NewVec2D(newRoom.X+newRoom.Width, newRoom.Y+2+(rand.Int()%(newRoom.Height-4)))
			break
		case DirectionSouth:
			door = NewVec2D(newRoom.X+2+(rand.Int()%(newRoom.Width-4)), newRoom.Y+newRoom.Height)
			break
		}
		data.Set(door.X, door.Y, DoorTileType)

		newRoom.AddDoor(direction, door)
		// opposite direction
		adjacentRoom.AddDoor((direction+2)%4, door)
		//newRoom.IsConnected = true
		//adjacentRoom.IsConnected = true

		newRoom = adjacentRoom
	}
}

func abs(n int) int {
	return int(math.Abs(float64(n)))
}

// Create ...
func (strategy *RandomRoomStrategy) Create(data *DungeonData, mask AreaMask) {
	defer util.TimeTrack(time.Now(), "randomroomstrategy::Create")

	// update max rooms
	if strategy.MaxRooms < 0 {
		strategy.MaxRooms = getMaxRoomsForDensity(data, strategy.Density)
	}

	// 1st Step: Create rooms
	for i := 0; i < strategy.MaxRooms; i++ {

		if newRoom, err := strategy.createRandomRoom(data, mask); err == nil {
			if !strategy.roomCollidesWithExisting(data, newRoom) {
				addRoomToDungeon(data, newRoom)

				// (ChanceOfAdjacentRooms) chance to build the new room next to the old one
				for i := 0; chanceInPercent(strategy.ChanceOfAdjacentRooms) && i < 5; i++ {
					strategy.buildAdjacentRoom(data, newRoom)
				}
			}
		}
	}

	// 2nd Step: Create hallways between rooms
	for _, room := range data.Rooms {

		if room.IsConnected {
			continue
		}

		for i := 0; i < strategy.RoomConnectedness; i++ {
			// Select a random wall
			start, direction, wall, err := strategy.selectRandomWall(room)

			if err == nil {
				collision := false
				current := start

				// create a new door at the starting point
				data.Set(current.X, current.Y, DoorTileType)
				room.AddDoor(wall, current)

				//TODO: add exit/action to room (not roomdata)

				// walk direction until collision

				chanceForDivergence := strategy.ChanceForDivergence

				for !collision {
					next := current.Add(direction)

					if chanceInPercent(chanceForDivergence) {
						direction = changeDirection(direction)
						chanceForDivergence /= 2
					}

					//collided with dungeon bounds
					if !mask.IsInside(next.X, next.Y) {
						collision = true
						data.Set(current.X, current.Y, WallTileType)
						break
					}

					tileType := data.Get(next.X, next.Y)

					if tileType != EmptyTileType {
						collision = true
					} else {
						data.Set(next.X, next.Y, FloorTileType)
					}
					current = next
				}

				switch data.Get(current.X, current.Y) {
				case FloorTileType:
					room.IsConnected = true
					break
				case WallTileType:

					if connectedRoom, error := data.FindRoomForCoord(current.X, current.Y); error == nil {
						if !connectedRoom.IsCorner(current.X, current.Y) {
							foundDoor := false
							// check if there is already a door 1 unit away
							if abs(direction.X) == 1 {
								if data.Get(current.X, current.Y-1) == DoorTileType {
									// one step back
									current = current.Add(direction.Invert())
									data.Set(current.X, current.Y-1, FloorTileType)
									foundDoor = true
								} else if data.Get(current.X, current.Y+1) == DoorTileType {
									// one step back
									current = current.Add(direction.Invert())
									data.Set(current.X, current.Y+1, FloorTileType)
									foundDoor = true

								}
							} else if abs(direction.Y) == 1 {
								if data.Get(current.X-1, current.Y) == DoorTileType {
									// one step back
									current = current.Add(direction.Invert())
									data.Set(current.X-1, current.Y, FloorTileType)
									foundDoor = true

								} else if data.Get(current.X+1, current.Y) == DoorTileType {
									// one step back
									current = current.Add(direction.Invert())
									data.Set(current.X+1, current.Y, FloorTileType)
									foundDoor = true
								}
							}

							// no door yet, add a new one at the current position
							if !foundDoor {
								connectedWall, err := connectedRoom.GetWallForPosition(current.X, current.Y)

								if err == nil {
									data.Set(current.X, current.Y, DoorTileType)
									connectedRoom.IsConnected = true
									connectedRoom.AddDoor(connectedWall, current)
									room.IsConnected = true
								} else {
									log.Fatal("Found doortile not on wall")
								}
							}
							// TODO create a passage between the two rooms
						}
					}
					break
				case DoorTileType:

					// TODO: find according room
					if connectedRoom, error := data.FindRoomForCoord(current.X, current.Y); error == nil {

						if !connectedRoom.IsCorner(current.X, current.Y) {
							connectedWall, err := connectedRoom.GetWallForPosition(current.X, current.Y)

							if err == nil {
								connectedRoom.IsConnected = true
								connectedRoom.AddDoor(connectedWall, current)
								room.IsConnected = true
							} else {
								log.Fatal("Found doortile not on wall")
							}
							// TODO create a passage between the two rooms
						}
					}
					break
				}
			}
		}
	}
	// 3rd Step: create walls around empty floor tiles
	cleanupHallways(data, mask)
}

// change direction by 90° randomly
func changeDirection(direction Vec2D) Vec2D {
	if direction.X == 0 {
		if rand.Int()%2 == 0 {
			return NewVec2D(1, 0)
		} else {
			return NewVec2D(-1, 0)
		}
	} else {
		if rand.Int()%2 == 0 {
			return NewVec2D(0, 1)
		} else {
			return NewVec2D(0, -1)
		}
	}
}

func cleanupHallways(data *DungeonData, mask AreaMask) {
	for x := 0; x < data.Width; x++ {
		for y := 0; y < data.Height; y++ {
			// if there is an empty tile check if there is a floor tile around
			// if there is a floor tile transform this into a wall
			if data.Get(x, y) == EmptyTileType {
				if findConnectedTile(x, y, FloorTileType, data, mask) > 0 {
					data.Set(x, y, WallTileType)
				}
			}
		}
	}
}

func findConnectedTile(x, y int, tile TileType, data *DungeonData, mask AreaMask) int {
	found := 0
	for x2 := x - 1; x2 < x+2; x2++ {
		for y2 := y - 1; y2 < y+2; y2++ {
			// not center
			if x2 == 0 && y2 == 0 {
				continue
			}
			// inside dungeon bounds
			if mask.IsInside(x2, y2) {
				if data.Get(x2, y2) == tile {
					found++
				}
			}
		}
	}
	return found
}

func (strategy *RandomRoomStrategy) selectRandomWall(room *RoomData) (Vec2D, Vec2D, int, error) {

	if len(room.Doors()) > 3 {
		return NewVec2D(0, 0), NewVec2D(0, 0), -1, errors.New("could not find wall without door")
	}

	wall := rand.Int() % 4

	// only select wall that has no door
	for i := 0; room.HasDoor(wall) && i < 4; i++ {
		// select another door
		wall = (wall + 1) % 4
	}

	var start Vec2D
	var direction Vec2D

	switch wall {
	case DirectionWest:
		start = NewVec2D(room.X, room.Y+1+(rand.Int()%(room.Height-2)))
		direction = NewVec2D(-1, 0)
		break
	case DirectionNorth:
		start = NewVec2D(room.X+1+(rand.Int()%(room.Width-2)), room.Y)
		direction = NewVec2D(0, -1)
		break
	case DirectionEast:
		start = NewVec2D(room.X+room.Width, room.Y+1+(rand.Int()%(room.Height-2)))
		direction = NewVec2D(1, 0)
		break
	case DirectionSouth:
		start = NewVec2D(room.X+1+(rand.Int()%(room.Width-2)), room.Y+room.Height)
		direction = NewVec2D(0, 1)
		break
	}
	return start, direction, wall, nil
}

func (strategy *RandomRoomStrategy) roomCollidesWithExisting(data *DungeonData, room *RoomData) bool {

	extruded := room.Extrude(strategy.SpaceBetweenRooms)

	for _, r := range data.Rooms {
		// extrude rooms by 1 so we get some spacing between rooms
		if r.Collides(*extruded) {
			return true
		}
	}
	return false
}

func addRoomToDungeon(data *DungeonData, room *RoomData) {

	data.Rooms = append(data.Rooms, room)

	for x := room.X; x < room.X+room.Width+1; x++ {
		for y := room.Y; y < room.Y+room.Height+1; y++ {

			// is wall
			if x == room.X || y == room.Y || x == (room.X+room.Width) || y == (room.Y+room.Height) {
				data.Set(x, y, WallTileType)
			} else {
				data.Set(x, y, FloorTileType)
			}
		}
	}
}

func max(a int, b int) int {
	if a < b {
		return b
	}
	return a
}

func chanceInPercent(p int) bool {
	return rand.Int()%100 < p
}

func (strategy *RandomRoomStrategy) createDuplicateRoom(data *DungeonData, lastRoom *RoomData) (*RoomData, int) {

	var direction int
	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))

	w := max(strategy.MinRoomWidth, r.Int()%strategy.MaxRoomWidth)
	h := max(strategy.MinRoomHeight, r.Int()%strategy.MaxRoomHeight)
	x := max(0, (r.Int()%data.Width - w))
	y := max(0, (r.Int()%data.Height - h))

	switch r.Int() % 4 {
	case DirectionWest:
		h = lastRoom.Height
		if lastRoom.X-w > 0 {
			x = lastRoom.X - w
			y = lastRoom.Y
			direction = DirectionWest
		}
		break
	case DirectionNorth:
		w = lastRoom.Width
		if lastRoom.Y-h > 0 {
			x = lastRoom.X
			y = lastRoom.Y - h
			direction = DirectionNorth
		}
		break
	case DirectionEast:
		h = lastRoom.Height
		if (lastRoom.X+lastRoom.Width)+w < data.Width {
			x = lastRoom.X + lastRoom.Width
			y = lastRoom.Y
			direction = DirectionEast
		}
		break
	case DirectionSouth:
		w = lastRoom.Width
		if (lastRoom.Y+lastRoom.Height)+h < data.Height {
			x = lastRoom.X
			y = lastRoom.Y + lastRoom.Height
			direction = DirectionSouth
		}
		break

	}

	return NewRoomData(x, y, w, h), direction
}

func (strategy *RandomRoomStrategy) createRandomRoom(data *DungeonData, mask AreaMask) (*RoomData, error) {

	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))

	var w, h, x, y int
	isInside := false

	// max trys?
	for i := 0; i < 5 && !isInside; i++ {
		w = max(strategy.MinRoomWidth, r.Int()%strategy.MaxRoomWidth)
		h = max(strategy.MinRoomHeight, r.Int()%strategy.MaxRoomHeight)
		x = max(0, (r.Int()%data.Width - w))
		y = max(0, (r.Int()%data.Height - h))

		isInside = mask.IsInside(x, y) && mask.IsInside(x+w, y) && mask.IsInside(x, y+h) && mask.IsInside(x+w, y+h)
	}

	if isInside {
		return NewRoomData(x, y, w, h), nil
	}

	return nil, errors.New("could not create random room")
}
