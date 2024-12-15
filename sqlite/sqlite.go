// Package sqlite stores pads and shifts in a SQLite database. Times are stored as unix timestamps, so we can easily compare them.
//
// Shift statements must be called with shift ids that are confirmed to belong to our pad (e. g. from GetShift).
// Taker statements must be called with taker ids that are confirmed to belong to the selected shift.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/wansing/shiftpad"
	"golang.org/x/exp/maps"
)

var ErrUnauthorized = errors.New("unauthorized")

type DB struct {
	SQLDB                *sql.DB
	addPad               *sql.Stmt
	addShare             *sql.Stmt
	addShift             *sql.Stmt
	addTaker             *sql.Stmt
	addTakerWithID       *sql.Stmt
	approveTake          *sql.Stmt
	deletePad            *sql.Stmt
	deletePads           *sql.Stmt
	deleteShift          *sql.Stmt
	deleteShifts         *sql.Stmt
	deleteTakers         *sql.Stmt
	getPad               *sql.Stmt
	getShare             *sql.Stmt
	getShares            *sql.Stmt
	getShift             *sql.Stmt
	getShifts            *sql.Stmt
	getShiftsByEvent     *sql.Stmt
	getTakerNames        *sql.Stmt
	getTakersByShift     *sql.Stmt
	getTakesByName       *sql.Stmt
	setPaidOut           *sql.Stmt
	updatePad            *sql.Stmt
	updatePadLastUpdated *sql.Stmt
	updateShift          *sql.Stmt
	updateShiftModified  *sql.Stmt
}

func OpenDB(dbpath string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", dbpath+"?_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared&_foreign_keys=true") // _foreign_keys=true is important
	if err != nil {
		return nil, err
	}

	var db = &DB{
		SQLDB: sqlDB,
	}

	if _, err := sqlDB.Exec(`
		create table if not exists pad (
			id           text primary key,
			description  text not null,
			ical_overlay text not null,
			last_updated text not null,
			location     text not null,
			name         text not null,
			shift_names  text not null
		);
		create table if not exists share (
			secret text primary key,
			pad    text not null,
			auth   text not null
		);
		create table if not exists shift (
			id            integer primary key,
			pad           text    not null,
			modified      integer not null,
			name          text    not null,
			note          text    not null,
			paid          boolean not null,
			event         text    not null,
			quantity      integer not null,
			begin         integer not null,
			end           integer not null,
			foreign key (pad) references pad(id) on update cascade on delete cascade
		);
		create table if not exists taker (
			id       integer primary key,
			pad      text    not null, -- for easy select
			shift    integer not null,
			name     text    not null,
			contact  text    not null,
			approved boolean not null,
			paid_out boolean not null,
			foreign key (shift) references shift(id) on update cascade on delete cascade
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
			shift_names
		) values (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	db.addShare, err = sqlDB.Prepare(`
		insert into share (
			secret,
			pad,
			auth
		) values (?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	db.addShift, err = sqlDB.Prepare(`
		insert into shift (
			pad,
			modified,
			name,
			note,
			paid,
			event,
			quantity,
			begin,
			end
		) values (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	db.addTaker, err = sqlDB.Prepare(`
		insert into taker (
			pad,
			shift,
			name,
			contact,
			approved,
			paid_out
		) values (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	db.addTakerWithID, err = sqlDB.Prepare(`
		insert into taker (
			id,
			pad,
			shift,
			name,
			contact,
			approved,
			paid_out
		) values (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	db.approveTake, err = sqlDB.Prepare(`
		update taker
		set approved = true
		where id = ?
			and shift = ?
	`)
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
		where id = ?`)
	if err != nil {
		return nil, err
	}
	db.deleteShifts, err = sqlDB.Prepare(`
		delete from shift
		where pad = ?`)
	if err != nil {
		return nil, err
	}
	db.deleteTakers, err = sqlDB.Prepare(`
		delete from taker
		where shift = ?`)
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
			shift_names
		from pad
		where id = ?
		limit 1`)
	if err != nil {
		return nil, err
	}
	db.getShare, err = sqlDB.Prepare(`
		select auth
		from share
		where secret = ?
			and pad = ?`)
	if err != nil {
		return nil, err
	}
	db.getShares, err = sqlDB.Prepare(`
		select
			secret,
			auth
		from share
		where pad = ?`)
	if err != nil {
		return nil, err
	}
	db.getShift, err = sqlDB.Prepare(`
		select
			id,
			modified,
			name,
			note,
			paid,
			event,
			quantity,
			begin,
			end
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
			paid,
			event,
			quantity,
			begin,
			end
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
			paid,
			event,
			quantity,
			begin,
			end
		from shift
		where pad = ?
			and event = ?`)
	if err != nil {
		return nil, err
	}
	db.getTakerNames, err = sqlDB.Prepare(`
		select distinct taker.name
		from shift, taker
		where shift.id = taker.shift
			and (shift.paid = true or taker.paid_out = true)
			and shift.pad = ?
		`)
	if err != nil {
		return nil, err
	}
	db.getTakersByShift, err = sqlDB.Prepare(`
		select
			id,
			name,
			contact,
			approved,
			paid_out
		from taker
		where shift = ?
	`)
	if err != nil {
		return nil, err
	}
	db.getTakesByName, err = sqlDB.Prepare(`
		select
			taker.id,
			taker.shift,
			taker.name,
			taker.contact,
			taker.approved,
			taker.paid_out
		from taker
		where taker.pad = ?
			and taker.name = ?
	`)
	if err != nil {
		return nil, err
	}
	db.setPaidOut, err = sqlDB.Prepare(`
		update taker
		set paid_out = ?
		where id = ?
	`)
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
			paid = ?,
			event = ?,
			quantity = ?,
			begin = ?,
			end = ?
		where id = ?`)
	if err != nil {
		return nil, err
	}
	db.updateShiftModified, err = sqlDB.Prepare(`
		update shift
		set modified = ?
		where id = ?
	`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) AddPad(pad shiftpad.Pad) error {
	shiftnames := strings.Join(pad.ShiftNames, "\n")
	_, err := db.addPad.Exec(pad.ID, pad.Description, pad.ICalOverlay, pad.LastUpdated, pad.Location.String(), pad.Name, shiftnames)
	return err
}

func (db *DB) AddShare(pad shiftpad.Pad, secret string, auth shiftpad.Auth) error {
	_, err := db.addShare.Exec(secret, pad.ID, auth.Encode())
	return err
}

func (db *DB) AddShift(pad *shiftpad.Pad, shift shiftpad.Shift) error {
	_, err := db.addShift.Exec(pad.ID, shift.Modified.Unix(), shift.Name, shift.Note, shift.Paid, shift.EventUID, shift.Quantity, shift.Begin.Unix(), shift.End.Unix())
	return err
}

func (db *DB) ApproveTake(shift *shiftpad.Shift, take shiftpad.Take) error {
	_, err := db.approveTake.Exec(take.ID, shift.ID)
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

func (db *DB) DeleteShift(shift *shiftpad.Shift) error {
	_, err := db.deleteShift.Exec(shift.ID)
	return err
}

func (db *DB) GetAuthPad(id, secret string) (shiftpad.AuthPad, error) {
	var pad = &shiftpad.Pad{}
	var location string
	var shiftnames string
	if err := db.getPad.QueryRow(id).Scan(&pad.ID, &pad.Description, &pad.ICalOverlay, &pad.LastUpdated, &location, &pad.Name, &shiftnames); err != nil {
		return shiftpad.AuthPad{}, err
	}
	loc, err := time.LoadLocation(location)
	if err != nil {
		loc = shiftpad.SystemLocation
	}
	pad.Location = loc
	pad.ShiftNames = strings.FieldsFunc(shiftnames, func(r rune) bool { return r == '\r' || r == '\n' })

	var authstr string
	if err := db.getShare.QueryRow(secret, pad.ID).Scan(&authstr); err != nil {
		return shiftpad.AuthPad{}, err
	}
	auth, err := shiftpad.DecodeAuth(authstr)
	if err != nil {
		return shiftpad.AuthPad{}, err
	}

	return shiftpad.AuthPad{
		Pad: pad,
		Share: shiftpad.Share{
			Auth:   auth,
			Secret: secret,
		},
	}, nil
}

func (db *DB) GetShares(pad *shiftpad.Pad) ([]shiftpad.Share, error) {
	rows, err := db.getShares.Query(pad.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []shiftpad.Share
	for rows.Next() {
		var secret string
		var authStr string
		if err := rows.Scan(&secret, &authStr); err != nil {
			return nil, err
		}
		auth, err := shiftpad.DecodeAuth(authStr)
		if err != nil {
			return nil, err
		}
		shares = append(shares, shiftpad.Share{
			Auth:   auth,
			Secret: secret,
		})
	}
	return shares, nil
}

func (db *DB) GetShift(pad *shiftpad.Pad, id int) (*shiftpad.Shift, error) {
	return db.readShift(pad, id, true)
}

func (db *DB) readShift(pad *shiftpad.Pad, id int, loadTakers bool) (*shiftpad.Shift, error) {
	var shift = &shiftpad.Shift{}
	var modified int64
	var begin int64
	var end int64
	if err := db.getShift.QueryRow(pad.ID, id).Scan(&shift.ID, &modified, &shift.Name, &shift.Note, &shift.Paid, &shift.EventUID, &shift.Quantity, &begin, &end); err != nil {
		return nil, err
	}
	shift.Modified = time.Unix(modified, 0).In(pad.Location)
	shift.Begin = time.Unix(begin, 0).In(pad.Location)
	shift.End = time.Unix(end, 0).In(pad.Location)

	if loadTakers {
		if takes, err := db.GetTakersByShift(shift.ID); err == nil {
			shift.Takes = takes
		} else {
			return nil, err
		}
	}

	return shift, nil
}

func (db *DB) GetShifts(pad *shiftpad.Pad, from, to int64) ([]shiftpad.Shift, error) {
	return db.readShifts(pad.Location, db.getShifts, pad.ID, from, to, from, to, from, to)
}

func (db *DB) GetShiftsByEvent(pad *shiftpad.Pad, eventUID string) ([]shiftpad.Shift, error) {
	return db.readShifts(pad.Location, db.getShiftsByEvent, pad.ID, eventUID)
}

func (db *DB) readShifts(location *time.Location, stmt *sql.Stmt, args ...any) ([]shiftpad.Shift, error) {
	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shifts []shiftpad.Shift
	for rows.Next() {
		var shift shiftpad.Shift
		var modified int64
		var begin int64
		var end int64
		if err := rows.Scan(&shift.ID, &modified, &shift.Name, &shift.Note, &shift.Paid, &shift.EventUID, &shift.Quantity, &begin, &end); err != nil {
			return nil, err
		}
		shift.Modified = time.Unix(modified, 0).In(location)
		shift.Begin = time.Unix(begin, 0).In(location)
		shift.End = time.Unix(end, 0).In(location)

		if takes, err := db.GetTakersByShift(shift.ID); err == nil {
			shift.Takes = takes
		} else {
			return nil, err
		}

		shifts = append(shifts, shift)
	}
	return shifts, nil
}

// returned shifts contain only takes with the given taker name
func (db *DB) GetTakesByTaker(pad *shiftpad.Pad, name string) ([]shiftpad.Shift, error) {
	rows, err := db.getTakesByName.Query(pad.ID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var takes = make(map[int][]shiftpad.Take) // key: shift id
	for rows.Next() {
		var take shiftpad.Take
		var shiftID int
		if err := rows.Scan(&take.ID, &shiftID, &take.Name, &take.Contact, &take.Approved, &take.PaidOut); err != nil {
			return nil, err
		}
		takes[shiftID] = append(takes[shiftID], take)
	}

	var shifts []shiftpad.Shift
	for _, shiftID := range maps.Keys(takes) {
		shift, err := db.readShift(pad, shiftID, false)
		if err != nil {
			return nil, err
		}
		shift.Takes = takes[shift.ID]
		shifts = append(shifts, *shift)
	}
	return shifts, nil
}

func (db *DB) GetTakerNames(pad *shiftpad.Pad) ([]string, error) {
	rows, err := db.getTakerNames.Query(pad.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var takerNames []string
	for rows.Next() {
		var takerName string
		if err := rows.Scan(&takerName); err != nil {
			return nil, err
		}
		takerNames = append(takerNames, takerName)
	}
	return takerNames, nil
}

func (db *DB) GetTakersByShift(shift int) ([]shiftpad.Take, error) {
	rows, err := db.getTakersByShift.Query(shift)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var takes []shiftpad.Take
	for rows.Next() {
		var take shiftpad.Take
		if err := rows.Scan(&take.ID, &take.Name, &take.Contact, &take.Approved, &take.PaidOut); err != nil {
			return nil, err
		}
		takes = append(takes, take)
	}
	return takes, nil
}

// SetPaidOut writes take.PaidOut to the database.
// Alternatively, we could delete and re-add the takes.
func (db *DB) SetPaidOut(takes []shiftpad.Take) error {
	tx, err := db.SQLDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, take := range takes {
		if _, err := tx.Stmt(db.setPaidOut).Exec(take.PaidOut, take.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) TakeShift(pad *shiftpad.Pad, shift *shiftpad.Shift, take shiftpad.Take) error {
	tx, err := db.SQLDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Stmt(db.addTaker).Exec(pad.ID, shift.ID, take.Name, take.Contact, take.Approved, take.PaidOut); err != nil {
		return err
	}
	if _, err := tx.Stmt(db.updateShiftModified).Exec(time.Now().Unix(), shift.ID); err != nil {
		return err
	}
	return tx.Commit()
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
	tx, err := db.SQLDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Stmt(db.updateShift).Exec(shift.Modified.Unix(), shift.Name, shift.Note, shift.Paid, shift.EventUID, shift.Quantity, shift.Begin.Unix(), shift.End.Unix(), shift.ID); err != nil {
		return err
	}
	if _, err := tx.Stmt(db.deleteTakers).Exec(shift.ID); err != nil {
		return err
	}
	for _, take := range shift.Takes {
		if take.ID > 0 {
			_, err = tx.Stmt(db.addTakerWithID).Exec(take.ID, pad.ID, shift.ID, take.Name, take.Contact, take.Approved, take.PaidOut)
		} else {
			_, err = tx.Stmt(db.addTaker).Exec(pad.ID, shift.ID, take.Name, take.Contact, take.Approved, take.PaidOut)
		}
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
