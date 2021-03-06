// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License included
// in the file licenses/BSL.txt and at www.mariadb.com/bsl11.
//
// Change Date: 2022-10-01
//
// On the date above, in accordance with the Business Source License, use
// of this software will be governed by the Apache License, Version 2.0,
// included in the file licenses/APL.txt and at
// https://www.apache.org/licenses/LICENSE-2.0

package row

import (
	"sort"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlbase"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/pkg/errors"
)

// rowHelper has the common methods for table row manipulations.
type rowHelper struct {
	TableDesc *sqlbase.ImmutableTableDescriptor
	// Secondary indexes.
	Indexes      []sqlbase.IndexDescriptor
	indexEntries []sqlbase.IndexEntry

	// Computed during initialization for pretty-printing.
	primIndexValDirs []encoding.Direction
	secIndexValDirs  [][]encoding.Direction

	// Computed and cached.
	primaryIndexKeyPrefix []byte
	primaryIndexCols      map[sqlbase.ColumnID]struct{}
	sortedColumnFamilies  map[sqlbase.FamilyID][]sqlbase.ColumnID
}

func newRowHelper(
	desc *sqlbase.ImmutableTableDescriptor, indexes []sqlbase.IndexDescriptor,
) rowHelper {
	rh := rowHelper{TableDesc: desc, Indexes: indexes}

	// Pre-compute the encoding directions of the index key values for
	// pretty-printing in traces.
	rh.primIndexValDirs = sqlbase.IndexKeyValDirs(&rh.TableDesc.PrimaryIndex)

	rh.secIndexValDirs = make([][]encoding.Direction, len(rh.Indexes))
	for i := range rh.Indexes {
		rh.secIndexValDirs[i] = sqlbase.IndexKeyValDirs(&rh.Indexes[i])
	}

	return rh
}

// encodeIndexes encodes the primary and secondary index keys. The
// secondaryIndexEntries are only valid until the next call to encodeIndexes or
// encodeSecondaryIndexes.
func (rh *rowHelper) encodeIndexes(
	colIDtoRowIndex map[sqlbase.ColumnID]int, values []tree.Datum,
) (primaryIndexKey []byte, secondaryIndexEntries []sqlbase.IndexEntry, err error) {
	if rh.primaryIndexKeyPrefix == nil {
		rh.primaryIndexKeyPrefix = sqlbase.MakeIndexKeyPrefix(rh.TableDesc.TableDesc(),
			rh.TableDesc.PrimaryIndex.ID)
	}
	primaryIndexKey, _, err = sqlbase.EncodeIndexKey(
		rh.TableDesc.TableDesc(), &rh.TableDesc.PrimaryIndex, colIDtoRowIndex, values, rh.primaryIndexKeyPrefix)
	if err != nil {
		return nil, nil, err
	}
	secondaryIndexEntries, err = rh.encodeSecondaryIndexes(colIDtoRowIndex, values)
	if err != nil {
		return nil, nil, err
	}
	return primaryIndexKey, secondaryIndexEntries, nil
}

// encodeSecondaryIndexes encodes the secondary index keys. The
// secondaryIndexEntries are only valid until the next call to encodeIndexes or
// encodeSecondaryIndexes.
func (rh *rowHelper) encodeSecondaryIndexes(
	colIDtoRowIndex map[sqlbase.ColumnID]int, values []tree.Datum,
) (secondaryIndexEntries []sqlbase.IndexEntry, err error) {
	if len(rh.indexEntries) != len(rh.Indexes) {
		rh.indexEntries = make([]sqlbase.IndexEntry, len(rh.Indexes))
	}
	rh.indexEntries, err = sqlbase.EncodeSecondaryIndexes(
		rh.TableDesc.TableDesc(), rh.Indexes, colIDtoRowIndex, values, rh.indexEntries)
	if err != nil {
		return nil, err
	}
	return rh.indexEntries, nil
}

// skipColumnInPK returns true if the value at column colID does not need
// to be encoded because it is already part of the primary key. Composite
// datums are considered too, so a composite datum in a PK will return false.
// TODO(dan): This logic is common and being moved into TableDescriptor (see
// #6233). Once it is, use the shared one.
func (rh *rowHelper) skipColumnInPK(
	colID sqlbase.ColumnID, family sqlbase.FamilyID, value tree.Datum,
) (bool, error) {
	if rh.primaryIndexCols == nil {
		rh.primaryIndexCols = make(map[sqlbase.ColumnID]struct{})
		for _, colID := range rh.TableDesc.PrimaryIndex.ColumnIDs {
			rh.primaryIndexCols[colID] = struct{}{}
		}
	}
	if _, ok := rh.primaryIndexCols[colID]; !ok {
		return false, nil
	}
	if family != 0 {
		return false, errors.Errorf("primary index column %d must be in family 0, was %d", colID, family)
	}
	if cdatum, ok := value.(tree.CompositeDatum); ok {
		// Composite columns are encoded in both the key and the value.
		return !cdatum.IsComposite(), nil
	}
	// Skip primary key columns as their values are encoded in the key of
	// each family. Family 0 is guaranteed to exist and acts as a
	// sentinel.
	return true, nil
}

func (rh *rowHelper) sortedColumnFamily(famID sqlbase.FamilyID) ([]sqlbase.ColumnID, bool) {
	if rh.sortedColumnFamilies == nil {
		rh.sortedColumnFamilies = make(map[sqlbase.FamilyID][]sqlbase.ColumnID, len(rh.TableDesc.Families))
		for i := range rh.TableDesc.Families {
			family := &rh.TableDesc.Families[i]
			colIDs := append([]sqlbase.ColumnID(nil), family.ColumnIDs...)
			sort.Sort(sqlbase.ColumnIDs(colIDs))
			rh.sortedColumnFamilies[family.ID] = colIDs
		}
	}
	colIDs, ok := rh.sortedColumnFamilies[famID]
	return colIDs, ok
}
