package parser

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/franela/goblin"
)

func TestPlugin(t *testing.T) {
	g := goblin.Goblin(t)

	g.Describe("Parse", func() {
		file, err := os.Open("fixture.txt")
		if err != nil {
			g.Fail("Cannot open the fixture file")
		}

		b, err := ioutil.ReadAll(file)
		if err != nil {
			g.Fail("Cannot read the fixture file")
		}

		g.It("fails when mode is invalid", func() {
			pa := &Parser{
				Mode:    "invalid",
				Message: string(b),
			}
			_, err := Parse(pa)
			g.Assert(err != nil).IsTrue("should have received error of passing in invalid mode")
		})

		g.It("parses message in summary mode", func() {
			pa := &Parser{
				Mode:    "summary",
				Message: string(b),
			}
			out, _ := Parse(pa)
			g.Assert(out).Equal(`Plan: 3 to add, 0 to change, 0 to destroy.
`)
		})
		g.It("parses message in simple mode", func() {
			pa := &Parser{
				Mode:    "simple",
				Message: string(b),
			}
			out, _ := Parse(pa)
			g.Assert(out).Equal(`# module.saml_data_analyst.aws_iam_role_policy_attachment.prod_attach[0] will be created
# module.saml_data_analyst.aws_iam_role_policy_attachment.prod_aws_attach[0] will be created
# module.saml_data_engineer.aws_iam_role_policy_attachment.prod_attach[0] will be created

Plan: 3 to add, 0 to change, 0 to destroy.
`)
		})
		g.It("parses message in full mode", func() {
			pa := &Parser{
				Mode:    "full",
				Message: string(b),
			}
			out, _ := Parse(pa)
			g.Assert(out).Equal(strings.ReplaceAll(`An execution plan has been generated and is shown below.
Resource actions are indicated with the following symbols:
+ create

Terraform will perform the following actions:

# module.saml_data_analyst.aws_iam_role_policy_attachment.prod_attach[0] will be created
+ resource "aws_iam_role_policy_attachment" "prod_attach" {
			+ id         = (known after apply)
			+ policy_arn = (known after apply)
			+ role       = "DataAnalyst"
		}

# module.saml_data_analyst.aws_iam_role_policy_attachment.prod_aws_attach[0] will be created
+ resource "aws_iam_role_policy_attachment" "prod_aws_attach" {
			+ id         = (known after apply)
			+ policy_arn = "arn:aws:iam::aws:policy/ReadOnlyAccess"
			+ role       = "DataAnalyst"
		}

# module.saml_data_engineer.aws_iam_role_policy_attachment.prod_attach[0] will be created
+ resource "aws_iam_role_policy_attachment" "prod_attach" {
			+ id         = (known after apply)
			+ policy_arn = (known after apply)
			+ role       = "DataEngineer"
		}

Plan: 3 to add, 0 to change, 0 to destroy.
`, "\t", "  "))
		})
	})
}
