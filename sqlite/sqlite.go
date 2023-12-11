// Package sqlite stores pads and shifts in a SQLite database. Times are stored as unix timestamps, so we can easily compare them.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/wansing/shiftpad"
)

var ErrUnauthorized = errors.New("unauthorized")

type DB struct {
	SQLDB                *sql.DB
	addPad               *sql.Stmt
	addShift             *sql.Stmt
	deletePad            *sql.Stmt
	deletePads           *sql.Stmt
	deleteShift          *sql.Stmt
	deleteShifts         *sql.Stmt
	getMaxShiftID        *sql.Stmt
	getPad               *sql.Stmt
	getShift             *sql.Stmt
	getShifts            *sql.Stmt
	getShiftsByEvent     *sql.Stmt
	takeShift            *sql.Stmt
	updatePad            *sql.Stmt
	updatePadLastUpdated *sql.Stmt
	updateShift          *sql.Stmt
}

func OpenDB(dbpath string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", dbpath+"?_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared&_foreign_keys=true") // _foreign_keys=true is important
	if err != nil {
		return nil, fmt.Errorf("error opening database %s: %w", dbpath, err)
	}

	var db = &DB{
		SQLDB: sqlDB,
	}

	if _, err := sqlDB.Exec(`
		create table if not exists pad (
			id           text not null primary key,
			description  text not null,
			ical_overlay text not null,
			last_updated text not null,
			location     text not null,
			name         text not null,
			private_key  text not null,
			shift_names  text not null
		);
		create table if not exists shift (
			pad           text    not null,
			id            integer not null,
			modified      integer not null,
			name          text    not null,
			note          text    not null,
			event         text    not null,
			begin         integer not null,
			end           integer not null,
			taker_name    text    not null,
			taker_contact text    not null,
			foreign key (pad) references pad(id) on update cascade on delete cascade,
			primary key (pad, id)
		);

		create index if not exists last_updated_index on pad(last_updated);
		create index if not exists pad_index          on shift(pad);
		create index if not exists pad_begin_index    on shift(pad, begin);
		create index if not exists pad_end_index      on shift(pad, end);
		create index if not exists pad_event_index    on shift(pad, event);
	`); err != nil {
		return nil, err
	}

	db.addPad, err = sqlDB.Prepare(`
		insert into pad (
			id,
			description,
			ical_overlay,
			last_updated,
			location,
			name,
			private_key,
			shift_names
		) values (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	db.addShift, err = sqlDB.Prepare(`
		insert into shift (
			pad,
			id,
			modified,
			name,
			note,
			event,
			begin,
			end,
			taker_name,
			taker_contact
		) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	db.deletePad, err = sqlDB.Prepare(`
		delete from pad
		where id = ?`)
	if err != nil {
		return nil, err
	}
	db.deletePads, err = sqlDB.Prepare(`
		delete from pad
		where last_updated < ?`)
	if err != nil {
		return nil, err
	}
	db.deleteShift, err = sqlDB.Prepare(`
		delete from shift
		where pad = ?
			and id = ?`)
	if err != nil {
		return nil, err
	}
	db.deleteShifts, err = sqlDB.Prepare(`
		delete from shift
		where pad = ?`)
	if err != nil {
		return nil, err
	}
	db.getMaxShiftID, err = sqlDB.Prepare(`
		select ifnull(max(id), 0)
		from shift
		where pad = ?`)
	if err != nil {
		return nil, err
	}
	db.getPad, err = sqlDB.Prepare(`
		select
			id,
			description,
			ical_overlay,
			last_updated,
			location,
			name,
			private_key,
			shift_names
		from pad
		where id = ?
		limit 1`)
	if err != nil {
		return nil, err
	}
	db.getShift, err = sqlDB.Prepare(`
		select
			id,
			modified,
			name,
			note,
			event,
			begin,
			end,
			taker_name,
			taker_contact
		from shift
		where pad = ?
			and id = ?`)
	if err != nil {
		return nil, err
	}
	db.getShifts, err = sqlDB.Prepare(`
		select
			id,
			modified,
			name,
			note,
			event,
			begin,
			end,
			taker_name,
			taker_contact
		from shift
		where pad = ?
			and (
				(begin >= ? and begin < ?)
				or (end >= ? and end < ?)
				or (begin != 0 and begin < ? and end >= ?)
			)`)
	if err != nil {
		return nil, err
	}
	db.getShiftsByEvent, err = sqlDB.Prepare(`
		select
			id,
			modified,
			name,
			note,
			event,
			begin,
			end,
			taker_name,
			taker_contact
		from shift
		where pad = ?
			and event = ?`)
	if err != nil {
		return nil, err
	}
	db.takeShift, err = sqlDB.Prepare(`
		update shift
		set
			modified = ?,
			taker_name = ?,
			taker_contact = ?
		where pad = ?
			and id = ?`)
	if err != nil {
		return nil, err
	}
	db.updatePad, err = sqlDB.Prepare(`
		update pad
		set
			description = ?,
			ical_overlay = ?,
			location = ?,
			name = ?,
			shift_names = ?
		where id = ?`)
	if err != nil {
		return nil, err
	}
	db.updatePadLastUpdated, err = sqlDB.Prepare(`
		update pad
		set last_updated = ?
		where id = ?`)
	if err != nil {
		return nil, err
	}
	db.updateShift, err = sqlDB.Prepare(`
		update shift
		set
			modified = ?,
			name = ?,
			note = ?,
			event = ?,
			begin = ?,
			end = ?,
			taker_name = ?,
			taker_contact = ?
		where pad = ?
			and id = ?`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) AddPad(pad shiftpad.Pad) error {
	shiftnames := strings.Join(pad.ShiftNames, "\n")
	_, err := db.addPad.Exec(pad.ID, pad.Description, pad.ICalOverlay, pad.LastUpdated, pad.Location.String(), pad.Name, pad.PrivateKey, shiftnames)
	return err
}

func (db *DB) AddShift(pad *shiftpad.Pad, shift shiftpad.Shift) error {
	// auto_increment doesn't work with composite primary key, so we do this manually
	var maxShiftID int64
	if err := db.getMaxShiftID.QueryRow(pad.ID).Scan(&maxShiftID); err != nil {
		return err
	}
	_, err := db.addShift.Exec(pad.ID, maxShiftID+1, shift.Modified.Unix(), shift.Name, shift.Note, shift.EventUID, shift.Begin.Unix(), shift.End.Unix(), shift.TakerName, shift.TakerContact)
	return err
}

func (db *DB) DeletePad(pad shiftpad.Pad) error {
	if _, err := db.deletePad.Exec(pad.ID); err != nil {
		return err
	}
	_, err := db.deleteShifts.Exec(pad.ID)
	return err
}

func (db *DB) DeletePads(cutoff string) error {
	// validate cutoff
	if cutoff == "" || cutoff > time.Now().AddDate(0, 0, -60).Format(time.DateOnly) {
		return fmt.Errorf("invalid cutoff: %s", cutoff)
	}

	_, err := db.deletePads.Exec(cutoff)
	return err
}

func (db *DB) DeleteShift(pad *shiftpad.Pad, shift *shiftpad.Shift) error {
	_, err := db.deleteShift.Exec(pad.ID, shift.ID)
	return err
}

func (db *DB) GetPad(id string) (*shiftpad.Pad, error) {
	var pad = &shiftpad.Pad{}
	var location string
	var shiftnames string
	if err := db.getPad.QueryRow(id).Scan(&pad.ID, &pad.Description, &pad.ICalOverlay, &pad.LastUpdated, &location, &pad.Name, &pad.PrivateKey, &shiftnames); err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(location)
	if err != nil {
		loc = shiftpad.SystemLocation
	}
	pad.Location = loc
	pad.ShiftNames = strings.FieldsFunc(shiftnames, func(r rune) bool { return r == '\r' || r == '\n' })
	return pad, nil
}

func (db *DB) GetShift(pad *shiftpad.Pad, id int) (*shiftpad.Shift, error) {
	var shift = &shiftpad.Shift{}
	var modified int64
	var begin int64
	var end int64
	if err := db.getShift.QueryRow(pad.ID, id).Scan(&shift.ID, &modified, &shift.Name, &shift.Note, &shift.EventUID, &begin, &end, &shift.TakerName, &shift.TakerContact); err != nil {
		return nil, err
	}
	shift.Modified = time.Unix(modified, 0).In(pad.Location)
	shift.Begin = time.Unix(begin, 0).In(pad.Location)
	shift.End = time.Unix(end, 0).In(pad.Location)
	return shift, nil
}

func (db *DB) GetShifts(pad *shiftpad.Pad, from, to int64) ([]shiftpad.Shift, error) {
	rows, err := db.getShifts.Query(pad.ID, from, to, from, to, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return db.readShifts(rows, pad.Location)
}

func (db *DB) GetShiftsByEvent(pad *shiftpad.Pad, eventUID string) ([]shiftpad.Shift, error) {
	rows, err := db.getShiftsByEvent.Query(pad.ID, eventUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return db.readShifts(rows, pad.Location)
}

func (db *DB) readShifts(rows *sql.Rows, location *time.Location) ([]shiftpad.Shift, error) {
	var shifts []shiftpad.Shift
	for rows.Next() {
		var shift shiftpad.Shift
		var modified int64
		var begin int64
		var end int64
		if err := rows.Scan(&shift.ID, &modified, &shift.Name, &shift.Note, &shift.EventUID, &begin, &end, &shift.TakerName, &shift.TakerContact); err != nil {
			return nil, err
		}
		shift.Modified = time.Unix(modified, 0).In(location)
		shift.Begin = time.Unix(begin, 0).In(location)
		shift.End = time.Unix(end, 0).In(location)
		shifts = append(shifts, shift)
	}
	return shifts, nil
}

func (db *DB) TakeShift(pad *shiftpad.Pad, shift *shiftpad.Shift) error {
	_, err := db.takeShift.Exec(shift.Modified.Unix(), shift.TakerName, shift.TakerContact, pad.ID, shift.ID)
	return err
}

func (db *DB) UpdatePad(pad *shiftpad.Pad) error {
	shiftnames := strings.Join(pad.ShiftNames, "\n")
	_, err := db.updatePad.Exec(pad.Description, pad.ICalOverlay, pad.Location.String(), pad.Name, shiftnames, pad.ID)
	return err
}

func (db *DB) UpdatePadLastUpdated(pad *shiftpad.Pad, lastUpdated string) error {
	_, err := db.updatePadLastUpdated.Exec(lastUpdated, pad.ID)
	return err
}

func (db *DB) UpdateShift(pad *shiftpad.Pad, shift *shiftpad.Shift) error {
	_, err := db.updateShift.Exec(shift.Modified.Unix(), shift.Name, shift.Note, shift.EventUID, shift.Begin.Unix(), shift.End.Unix(), shift.TakerName, shift.TakerContact, pad.ID, shift.ID)
	return err
}
