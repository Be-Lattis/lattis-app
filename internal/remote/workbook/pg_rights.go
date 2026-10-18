package workbook

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Droits d'un classeur par l'API SQL du serveur (contrat client de lattisd,
// §5 et §7 ; schema/0005_advanced_rights.sql, 0006_admin.sql,
// 0009_droits.sql, 0012_teams.sql). Écritures sur rw, une transaction par appel.

func (s pgServer) tx(ctx context.Context, fn func(pgx.Tx) error) error {
	return s.with(ctx, usageWrite, func(c *pgx.Conn) error {
		return pgx.BeginFunc(ctx, c, fn)
	})
}

func (s pgServer) jsonCall(ctx context.Context, q string, out any, args ...any) error {
	return s.with(ctx, usageWrite, func(c *pgx.Conn) error {
		var raw []byte
		if err := c.QueryRow(ctx, q, args...).Scan(&raw); err != nil {
			return err
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%s : %w", q, err)
		}
		return nil
	})
}

func (s pgServer) ACL(ctx context.Context, workbookID string) (ACL, error) {
	var a ACL
	err := s.jsonCall(ctx, `SELECT lattis.workbook_acl($1::uuid)::text`, &a, workbookID)
	return a, err
}

func (s pgServer) Invite(ctx context.Context, workbookID, email string, roles []string) (string, error) {
	var id string
	err := s.tx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT lattis.invite($1, $2::uuid, $3::text[])::text`, email, workbookID, roles).Scan(&id)
	})
	return id, err
}

func (s pgServer) exec(ctx context.Context, q string, args ...any) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, q, args...)
		return err
	})
}

func (s pgServer) RevokeInvitation(ctx context.Context, id string) error {
	return s.exec(ctx, `SELECT lattis.revoke_invitation($1::uuid)`, id)
}

func (s pgServer) CreateRole(ctx context.Context, workbookID, name string) (string, error) {
	var id string
	err := s.tx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT lattis.create_role($1::uuid, $2)::text`, workbookID, name).Scan(&id)
	})
	return id, err
}

func (s pgServer) DropRole(ctx context.Context, roleID string) error {
	return s.exec(ctx, `SELECT lattis.drop_role($1::uuid)`, roleID)
}

// GrantRole : une personne ou une équipe (migration 0012), même fonction.
func (s pgServer) GrantRole(ctx context.Context, roleID, principal string) error {
	return s.exec(ctx, `SELECT lattis.grant_role($1::uuid, $2::uuid)`, roleID, principal)
}

func (s pgServer) RevokeRole(ctx context.Context, roleID, principal string) error {
	return s.exec(ctx, `SELECT lattis.revoke_role($1::uuid, $2::uuid)`, roleID, principal)
}

// Teams : lattis.list_teams (migration 0012) ; fonction absente (42883) :
// ErrUnsupported.
func (s pgServer) Teams(ctx context.Context) ([]Team, error) {
	out := []Team{}
	err := s.jsonCall(ctx, `SELECT lattis.list_teams()::text`, &out)
	if code, ok := Rejected(err); ok && code == "42883" {
		return nil, ErrUnsupported
	}
	return out, err
}

// nullable : chaîne vide → NULL.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s pgServer) SetLevels(ctx context.Context, levels []Level) error {
	for _, l := range levels {
		if err := CheckLevel(l); err != nil {
			return err
		}
	}
	return s.tx(ctx, func(tx pgx.Tx) error {
		for _, l := range levels {
			if _, err := tx.Exec(ctx, `SELECT lattis.set_level($1::uuid, $2, $3, $4, $5, $6)`,
				l.RoleID, l.NodeKind, l.NodeID, nullable(l.Level), nullable(l.ReadFilter), nullable(l.WriteFilter)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s pgServer) AddRestriction(ctx context.Context, workbookID string, r RestrictionRule) (string, error) {
	if err := CheckRestriction(r); err != nil {
		return "", err
	}
	roles := r.AppliesTo
	if roles == nil {
		roles = []string{}
	}
	var id string
	err := s.tx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT lattis.add_restriction($1::uuid, $2::uuid, $3, $4::uuid[], $5, $6)::text`,
			workbookID, r.TableID, r.Command, roles, r.Predicate, r.Label).Scan(&id)
	})
	return id, err
}

func (s pgServer) DropRestriction(ctx context.Context, id string) error {
	return s.exec(ctx, `SELECT lattis.drop_restriction($1::uuid)`, id)
}

func (s pgServer) Drift(ctx context.Context, workbookID string) ([]Drift, error) {
	out := []Drift{}
	err := s.jsonCall(ctx, `SELECT lattis.check_drift($1::uuid)::text`, &out, workbookID)
	return out, err
}

// Members : lattis.workbook_members (migration 0009) ; fonction absente
// (42883) : ErrUnsupported.
func (s pgServer) Members(ctx context.Context, workbookID string) ([]WorkbookMember, error) {
	out := []WorkbookMember{}
	err := s.jsonCall(ctx, `SELECT lattis.workbook_members($1::uuid)::text`, &out, workbookID)
	if code, ok := Rejected(err); ok && code == "42883" {
		return nil, ErrUnsupported
	}
	return out, err
}

func (s pgServer) Recompile(ctx context.Context, workbookID string) error {
	return s.exec(ctx, `SELECT lattis.compile_acl($1::uuid)`, workbookID)
}
