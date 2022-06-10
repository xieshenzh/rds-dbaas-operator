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
	"math/rand"
	"unicode"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/utils/pointer"
)

var _ = Describe("DbUtils", func() {
	Context("Generate Username", func() {
		DescribeTable("checking generateUsername",
			func(engine string, username string) {
				u := generateUsername(engine)
				Expect(u).Should(Equal(username))
			},

			Entry("aurora", "aurora", "admin"),
			Entry("aurora-mysql", "aurora-mysql", "admin"),
			Entry("aurora-postgresql", "aurora-postgresql", "postgres"),
			Entry("custom-oracle-ee", "custom-oracle-ee", "admin"),
			Entry("mariadb", "mariadb", "admin"),
			Entry("mysql", "mysql", "admin"),
			Entry("oracle-ee", "oracle-ee", "admin"),
			Entry("oracle-ee-cdb", "oracle-ee-cdb", "admin"),
			Entry("oracle-se2", "oracle-se2", "admin"),
			Entry("oracle-se2-cdb", "oracle-se2-cdb", "admin"),
			Entry("postgres", "postgres", "postgres"),
			Entry("sqlserver-ee", "sqlserver-ee", "admin"),
			Entry("sqlserver-se", "sqlserver-se", "admin"),
			Entry("sqlserver-ex", "sqlserver-ex", "admin"),
			Entry("sqlserver-web", "sqlserver-web", "admin"),
		)
	})

	Context("Generate DB Name", func() {
		DescribeTable("checking generateDBName",
			func(engine string, dbName *string) {
				n := generateDBName(engine)
				if dbName == nil {
					Expect(n).Should(BeNil())
				} else {
					Expect(*n).Should(Equal(*dbName))
				}
			},

			Entry("aurora", "aurora", pointer.String("mydb")),
			Entry("aurora-mysql", "aurora-mysql", pointer.String("mydb")),
			Entry("aurora-postgresql", "aurora-postgresql", pointer.String("postgres")),
			Entry("custom-oracle-ee", "custom-oracle-ee", pointer.String("ORCL")),
			Entry("mariadb", "mariadb", pointer.String("mydb")),
			Entry("mysql", "mysql", pointer.String("mydb")),
			Entry("oracle-ee", "oracle-ee", pointer.String("ORCL")),
			Entry("oracle-ee-cdb", "oracle-ee-cdb", pointer.String("ORCL")),
			Entry("oracle-se2", "oracle-se2", pointer.String("ORCL")),
			Entry("oracle-se2-cdb", "oracle-se2-cdb", pointer.String("ORCL")),
			Entry("postgres", "postgres", pointer.String("postgres")),
			Entry("sqlserver-ee", "sqlserver-ee", nil),
			Entry("sqlserver-se", "sqlserver-se", nil),
			Entry("sqlserver-ex", "sqlserver-ex", nil),
			Entry("sqlserver-web", "sqlserver-web", nil),
			Entry("Invalid", "invalid", nil),
		)
	})

	Context("Generate Binding Type", func() {
		DescribeTable("checking generateBindingType",
			func(engine string, bt string) {
				t := generateBindingType(engine)
				Expect(t).Should(Equal(bt))
			},

			Entry("aurora", "aurora", "mysql"),
			Entry("aurora-mysql", "aurora-mysql", "mysql"),
			Entry("aurora-postgresql", "aurora-postgresql", "postgresql"),
			Entry("custom-oracle-ee", "custom-oracle-ee", "oracle"),
			Entry("mariadb", "mariadb", "mysql"),
			Entry("mysql", "mysql", "mysql"),
			Entry("oracle-ee", "oracle-ee", "oracle"),
			Entry("oracle-ee-cdb", "oracle-ee-cdb", "oracle"),
			Entry("oracle-se2", "oracle-se2", "oracle"),
			Entry("oracle-se2-cdb", "oracle-se2-cdb", "oracle"),
			Entry("postgres", "postgres", "postgresql"),
			Entry("sqlserver-ee", "sqlserver-ee", "sqlserver"),
			Entry("sqlserver-se", "sqlserver-se", "sqlserver"),
			Entry("sqlserver-ex", "sqlserver-ex", "sqlserver"),
			Entry("sqlserver-web", "sqlserver-web", "sqlserver"),
		)
	})

	Context("Generate Default DB Name", func() {
		DescribeTable("checking getDefaultDBName",
			func(engine string, dbName *string) {
				n := getDefaultDBName(engine)
				if dbName == nil {
					Expect(n).Should(BeNil())
				} else {
					Expect(*n).Should(Equal(*dbName))
				}
			},

			Entry("aurora", "aurora", pointer.String("mysql")),
			Entry("aurora-mysql", "aurora-mysql", pointer.String("mysql")),
			Entry("aurora-postgresql", "aurora-postgresql", pointer.String("postgres")),
			Entry("custom-oracle-ee", "custom-oracle-ee", pointer.String("ORCL")),
			Entry("mariadb", "mariadb", pointer.String("mysql")),
			Entry("mysql", "mysql", pointer.String("mysql")),
			Entry("oracle-ee", "oracle-ee", pointer.String("ORCL")),
			Entry("oracle-ee-cdb", "oracle-ee-cdb", pointer.String("ORCL")),
			Entry("oracle-se2", "oracle-se2", pointer.String("ORCL")),
			Entry("oracle-se2-cdb", "oracle-se2-cdb", pointer.String("ORCL")),
			Entry("postgres", "postgres", pointer.String("postgres")),
			Entry("sqlserver-ee", "sqlserver-ee", pointer.String("master")),
			Entry("sqlserver-se", "sqlserver-se", pointer.String("master")),
			Entry("sqlserver-ex", "sqlserver-ex", pointer.String("master")),
			Entry("sqlserver-web", "sqlserver-web", pointer.String("master")),
			Entry("Invalid", "invalid", nil),
		)
	})

	Context("Generate Password", func() {
		DescribeTable("checking generatePassword",
			func() {
				s := generatePassword()
				Expect(len(s)).Should(BeNumerically("==", 12))
				special := false
				number := false
				letter := false
				unexpected := false
				for _, c := range s {
					switch {
					case unicode.IsNumber(c):
						number = true
					case unicode.IsPunct(c) || unicode.IsSymbol(c):
						special = true
					case unicode.IsLetter(c):
						letter = true
					default:
						unexpected = true
					}
				}
				Expect(special).Should(BeTrue())
				Expect(number).Should(BeTrue())
				Expect(letter).Should(BeTrue())
				Expect(unexpected).Should(BeFalse())
			},
			Entry("generating password 1"),
			Entry("generating password 2"),
			Entry("generating password 3"),
			Entry("generating password 4"),
			Entry("generating password 5"),
			Entry("generating password 6"),
			Entry("generating password 7"),
			Entry("generating password 8"),
			Entry("generating password 9"),
			Entry("generating password 10"),
		)
	})

	Context("Generate Random Int", func() {
		rand.Seed(GinkgoRandomSeed())

		DescribeTable("checking getRandInt",
			func(s int) {
				i := getRandInt(s)
				Expect(i).Should(BeNumerically(">=", 0))
				Expect(i).Should(BeNumerically("<", s))
			},
			Entry("random number 1", rand.Intn(10000)),
			Entry("random number 2", rand.Intn(10000)),
			Entry("random number 3", rand.Intn(10000)),
			Entry("random number 4", rand.Intn(10000)),
			Entry("random number 5", rand.Intn(10000)),
			Entry("random number 6", rand.Intn(10000)),
			Entry("random number 7", rand.Intn(10000)),
			Entry("random number 8", rand.Intn(10000)),
			Entry("random number 9", rand.Intn(10000)),
			Entry("random number 10", rand.Intn(10000)),
		)
	})
})
