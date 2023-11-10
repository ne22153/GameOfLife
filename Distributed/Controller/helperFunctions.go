package Controller

import "uk.ac.bris.cs/gameoflife/util"

func calculateAliveCells(world [][]byte) []util.Cell {
	var coordinates []util.Cell
	for index, row := range world {
		for index2 := range row {
			if world[index][index2] > 0 {
				coordinates = append(coordinates, util.Cell{X: index2, Y: index})
			}
		}
	}
	return coordinates
}
