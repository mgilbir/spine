package xlsx

import (
	"fmt"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// writeSheetPivotTables writes each of a sheet's session-added pivot tables:
// the pivot table part (xl/pivotTables/pivotTableN.xml) and its cache
// (xl/pivotCache/pivotCacheDefinitionN.xml + pivotCacheRecordsN.xml), together
// with the pivot table's relationship to its cache and the cache's
// relationship to its records. It appends the worksheet -> pivotTable
// relationships to sheetRels (returned) and records each cache in the
// workbook-level pending set so the workbook relationship and <pivotCaches>
// entry can be wired later. Content-type overrides are registered
// automatically by WritePart.
func (w *Workbook) writeSheetPivotTables(writer *opc.Writer, sheet *Sheet, sheetRels []*opc.Relationship, relUsed, used map[string]struct{}, pivotSeq, cacheSeq *int) ([]*opc.Relationship, error) {
	for _, pt := range sheet.newPivots {
		cachePart, cacheFile, recordsPart, recordsFile := allocPivotCacheNames(used, cacheSeq)
		tablePart, tableFile := allocPivotTableName(used, pivotSeq)

		// Records part.
		if err := writer.WritePart(recordsPart, opc.ContentTypePivotCacheRecords,
			oxml.MarshalPivotCacheRecords(pt.records)); err != nil {
			return nil, err
		}

		// Cache definition part, referencing its records via rId1.
		if err := writer.WritePart(cachePart, opc.ContentTypePivotCacheDefinition,
			oxml.MarshalPivotCacheDefinition(pt.cache, "rId1")); err != nil {
			return nil, err
		}
		if err := writer.WritePartRelationships(cachePart, []*opc.Relationship{{
			ID:         "rId1",
			Type:       opc.RelTypePivotCacheRecord,
			Target:     recordsFile,
			TargetMode: opc.TargetModeInternal,
		}}); err != nil {
			return nil, err
		}

		// Pivot table part, referencing its cache definition via rId1.
		if err := writer.WritePart(tablePart, opc.ContentTypePivotTable,
			oxml.MarshalPivotTableDefinition(pt.def)); err != nil {
			return nil, err
		}
		if err := writer.WritePartRelationships(tablePart, []*opc.Relationship{{
			ID:         "rId1",
			Type:       opc.RelTypePivotCacheDef,
			Target:     "../pivotCache/" + cacheFile,
			TargetMode: opc.TargetModeInternal,
		}}); err != nil {
			return nil, err
		}

		// Worksheet -> pivot table relationship.
		rid := fmt.Sprintf("rId%d", nextRelationshipID(relUsed))
		relUsed[rid] = struct{}{}
		sheetRels = append(sheetRels, &opc.Relationship{
			ID:         rid,
			Type:       opc.RelTypePivotTable,
			Target:     "../pivotTables/" + tableFile,
			TargetMode: opc.TargetModeInternal,
		})

		w.pendingPivotCaches = append(w.pendingPivotCaches, pendingPivotCache{
			cacheID: pt.cacheID,
			target:  "pivotCache/" + cacheFile,
		})
	}
	return sheetRels, nil
}

// finalizeWorkbookPivotCaches appends the workbook -> pivotCacheDefinition
// relationships for the caches written this save and populates the workbook's
// <pivotCaches> element with matching relationship ids. It returns the updated
// relationship slice. It is a no-op when no pivot caches are pending.
func (w *Workbook) finalizeWorkbookPivotCaches(wbRels []*opc.Relationship) []*opc.Relationship {
	if len(w.pendingPivotCaches) == 0 {
		return wbRels
	}
	usedIDs := make(map[string]struct{}, len(wbRels))
	for _, rel := range wbRels {
		if rel != nil {
			usedIDs[rel.ID] = struct{}{}
		}
	}
	caches := &oxml.CT_PivotCaches{}
	// Preserve any pivot caches the workbook already carried: their
	// workbook->pivotCacheDefinition relationships survive verbatim among wbRels,
	// so re-emitting the entries (with their original relationship ids) keeps the
	// existing pivots wired while the new caches are appended below.
	if existing := w.workbook.TakeExistingPivotCaches(); len(existing) > 0 {
		caches.Cache = append(caches.Cache, existing...)
	}
	for _, pc := range w.pendingPivotCaches {
		id := fmt.Sprintf("rId%d", nextRelationshipID(usedIDs))
		usedIDs[id] = struct{}{}
		wbRels = append(wbRels, &opc.Relationship{
			ID:         id,
			Type:       opc.RelTypePivotCacheDef,
			Target:     pc.target,
			TargetMode: opc.TargetModeInternal,
		})
		caches.Cache = append(caches.Cache, oxml.CT_PivotCache{CacheId: pc.cacheID, RID: id})
	}
	w.workbook.PivotCaches = caches
	w.workbook.EnsureChildOrder("pivotCaches")
	return wbRels
}

// allocPivotTableName finds a free /xl/pivotTables/pivotTableN.xml part.
func allocPivotTableName(used map[string]struct{}, seq *int) (partName, fileName string) {
	for {
		fileName = fmt.Sprintf("pivotTable%d.xml", *seq)
		partName = "/xl/pivotTables/" + fileName
		*seq++
		if _, ok := used[partName]; !ok {
			used[partName] = struct{}{}
			return partName, fileName
		}
	}
}

// allocPivotCacheNames finds a free index N such that both the cache definition
// and records parts (pivotCacheDefinitionN.xml / pivotCacheRecordsN.xml) are
// available, marking both used.
func allocPivotCacheNames(used map[string]struct{}, seq *int) (defPart, defFile, recPart, recFile string) {
	for {
		n := *seq
		*seq++
		defFile = fmt.Sprintf("pivotCacheDefinition%d.xml", n)
		recFile = fmt.Sprintf("pivotCacheRecords%d.xml", n)
		defPart = "/xl/pivotCache/" + defFile
		recPart = "/xl/pivotCache/" + recFile
		_, defUsed := used[defPart]
		_, recUsed := used[recPart]
		if !defUsed && !recUsed {
			used[defPart] = struct{}{}
			used[recPart] = struct{}{}
			return defPart, defFile, recPart, recFile
		}
	}
}
