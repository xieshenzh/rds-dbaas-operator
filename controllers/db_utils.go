/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	cryptorand "crypto/rand"
	"math/big"
	"math/rand"

	"k8s.io/utils/pointer"
)

const (
	postgres           = "postgres"
	auroraPostgresql   = "aurora-postgresql"
	mysql              = "mysql"
	mariadb            = "mariadb"
	aurora             = "aurora"
	auroraMysql        = "aurora-mysql"
	oracleSe2          = "oracle-se2"
	oracleSe2Cdb       = "oracle-se2-cdb"
	oracleEe           = "oracle-ee"
	oracleEeCdb        = "oracle-ee-cdb"
	customOracleEe     = "custom-oracle-ee"
	sqlserverEe        = "sqlserver-ee"
	sqlserverSe        = "sqlserver-se"
	sqlserverEx        = "sqlserver-ex"
	sqlserverWeb       = "sqlserver-web"
	customSqlserverEe  = "custom-sqlserver-ee"
	customSqlserverSe  = "custom-sqlserver-se"
	customSqlserverWeb = "custom-sqlserver-web"

	digits   = "0123456789"
	specials = "~=+%^*()[]{}!#$?|"
	letter   = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	all      = letter + digits + specials
)

func generateUsername(engine string) string {
	if engine == "postgres" || engine == "aurora-postgresql" {
		return "postgres"
	} else {
		return "admin"
	}
}

func generateDBName(engine string) *string {
	switch engine {
	case postgres, auroraPostgresql:
		return pointer.String("postgres")
	case mysql, mariadb, aurora, auroraMysql:
		return pointer.String("mydb")
	case oracleSe2, oracleSe2Cdb, oracleEe, oracleEeCdb, customOracleEe:
		return pointer.String("ORCL")
	default:
		return nil
	}
}

func generateBindingType(engine string) string {
	switch engine {
	case postgres, auroraPostgresql:
		return "postgresql"
	case mysql, mariadb, aurora, auroraMysql:
		return "mysql"
	case oracleSe2, oracleSe2Cdb, oracleEe, oracleEeCdb, customOracleEe:
		return "oracle"
	case sqlserverEe, sqlserverSe, sqlserverEx, sqlserverWeb, customSqlserverEe, customSqlserverSe, customSqlserverWeb:
		return "sqlserver"
	default:
		return ""
	}
}

func getDefaultDBName(engine string) *string {
	switch engine {
	case postgres, auroraPostgresql:
		return pointer.String("postgres")
	case sqlserverEe, sqlserverSe, sqlserverEx, sqlserverWeb, customSqlserverEe, customSqlserverSe, customSqlserverWeb:
		return pointer.String("master")
	case oracleSe2, oracleSe2Cdb, oracleEe, oracleEeCdb, customOracleEe:
		return pointer.String("ORCL")
	case mysql, mariadb, aurora, auroraMysql:
		return pointer.String("mysql")
	default:
		return nil
	}
}

func getDefaultDBPort(engine string) *int64 {
	switch engine {
	case postgres, auroraPostgresql:
		return pointer.Int64(5432)
	case sqlserverEe, sqlserverSe, sqlserverEx, sqlserverWeb, customSqlserverEe, customSqlserverSe, customSqlserverWeb:
		return pointer.Int64(1433)
	case oracleSe2, oracleSe2Cdb, oracleEe, oracleEeCdb, customOracleEe:
		return pointer.Int64(1521)
	case mysql, mariadb, aurora, auroraMysql:
		return pointer.Int64(3306)
	default:
		return nil
	}
}

func getDBEngineAbbreviation(engine *string) string {
	if engine == nil {
		return ""
	}

	switch *engine {
	case mysql:
		return "mysql-"
	case mariadb:
		return "mariadb-"
	case aurora:
		return "aurora-"
	case auroraMysql:
		return "aurora-mysql-"
	case postgres:
		return "postgres-"
	case auroraPostgresql:
		return "aurora-postgres-"
	case sqlserverEe:
		return "mssql-ee-"
	case sqlserverSe:
		return "mssql-se-"
	case sqlserverEx:
		return "mssql-ex-"
	case sqlserverWeb:
		return "mssql-web-"
	case customSqlserverEe:
		return "cust-mssql-ee-"
	case customSqlserverSe:
		return "cust-mssql-se-"
	case customSqlserverWeb:
		return "cust-mssql-web-"
	case oracleSe2:
		return "oracle-se2-"
	case oracleSe2Cdb:
		return "oracle-se2-cdb-"
	case oracleEe:
		return "oracle-ee-"
	case oracleEeCdb:
		return "oracle-ee-cdb-"
	case customOracleEe:
		return "cust-oracle-ee-"
	default:
		return ""
	}
}

func generatePassword() string {
	length := 12
	buf := make([]byte, length)
	buf[0] = digits[getRandInt(len(digits))]
	buf[1] = specials[getRandInt(len(specials))]
	buf[2] = letter[getRandInt(len(letter))]
	for i := 3; i < length; i++ {
		buf[i] = all[getRandInt(len(all))]
	}
	rand.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})
	return string(buf) // E.g. "3i[g0|)z"
}

func getRandInt(s int) int64 {
	result, _ := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(s)))
	return result.Int64()
}
