package intercept

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coreos/go-iptables/iptables"
)

const (
	nfqueueConnMarkAccept = 1001
	nfqueueConnMarkDrop   = 1002

	nftFamily = "inet"
	nftTable  = "gtopng"
)

type nftTableSpec struct {
	Defines       []string
	Family, Table string
	Chains        []nftChainSpec
}

func (t *nftTableSpec) String() string {
	chains := make([]string, 0, len(t.Chains))
	for _, c := range t.Chains {
		chains = append(chains, c.String())
	}
	return fmt.Sprintf(`
%s

table %s %s {
%s
}
`, strings.Join(t.Defines, "\n"), t.Family, t.Table, strings.Join(chains, ""))
}

type nftChainSpec struct {
	Chain  string
	Header string
	Rules  []string
}

func (c *nftChainSpec) String() string {
	return fmt.Sprintf(`
  chain %s {
    %s
    %s
  }
`, c.Chain, c.Header, strings.Join(c.Rules, "\n\x20\x20\x20\x20"))
}

type iptRule struct {
	Table, Chain string
	RuleSpec     []string
}

func generateNftRules(local, rst bool, nfqueueNum int) (*nftTableSpec, error) {
	if local && rst {
		return nil, fmt.Errorf("tcp rst is not supported in local mode")
	}
	table := &nftTableSpec{
		Family: nftFamily,
		Table:  nftTable,
	}
	table.Defines = append(table.Defines, fmt.Sprintf("define ACCEPT_CTMARK=%d", nfqueueConnMarkAccept))
	table.Defines = append(table.Defines, fmt.Sprintf("define DROP_CTMARK=%d", nfqueueConnMarkDrop))
	table.Defines = append(table.Defines, fmt.Sprintf("define QUEUE_NUM=%d", nfqueueNum))
	if local {
		table.Chains = []nftChainSpec{
			{Chain: "INPUT", Header: "type filter hook input priority filter; policy accept;"},
			{Chain: "OUTPUT", Header: "type filter hook output priority filter; policy accept;"},
		}
	} else {
		table.Chains = []nftChainSpec{
			{Chain: "FORWARD", Header: "type filter hook forward priority filter; policy accept;"},
		}
	}
	for i := range table.Chains {
		c := &table.Chains[i]
		c.Rules = append(c.Rules, "meta mark $ACCEPT_CTMARK ct mark set $ACCEPT_CTMARK")
		c.Rules = append(c.Rules, "ct mark $ACCEPT_CTMARK counter accept")
		if rst {
			c.Rules = append(c.Rules, "ip protocol tcp ct mark $DROP_CTMARK counter reject with tcp reset")
		}
		c.Rules = append(c.Rules, "ct mark $DROP_CTMARK counter drop")
		c.Rules = append(c.Rules, "counter queue num $QUEUE_NUM bypass")
	}
	return table, nil
}

func generateIptRules(local, rst bool, nfqueueNum int) ([]iptRule, error) {
	if local && rst {
		return nil, fmt.Errorf("tcp rst is not supported in local mode")
	}
	var chains []string
	if local {
		chains = []string{"INPUT", "OUTPUT"}
	} else {
		chains = []string{"FORWARD"}
	}
	rules := make([]iptRule, 0, 4*len(chains))
	for _, chain := range chains {
		rules = append(rules, iptRule{"filter", chain, []string{"-m", "mark", "--mark", strconv.Itoa(nfqueueConnMarkAccept), "-j", "CONNMARK", "--set-mark", strconv.Itoa(nfqueueConnMarkAccept)}})
		rules = append(rules, iptRule{"filter", chain, []string{"-m", "connmark", "--mark", strconv.Itoa(nfqueueConnMarkAccept), "-j", "ACCEPT"}})
		if rst {
			rules = append(rules, iptRule{"filter", chain, []string{"-p", "tcp", "-m", "connmark", "--mark", strconv.Itoa(nfqueueConnMarkDrop), "-j", "REJECT", "--reject-with", "tcp-reset"}})
		}
		rules = append(rules, iptRule{"filter", chain, []string{"-m", "connmark", "--mark", strconv.Itoa(nfqueueConnMarkDrop), "-j", "DROP"}})
		rules = append(rules, iptRule{"filter", chain, []string{"-j", "NFQUEUE", "--queue-num", strconv.Itoa(nfqueueNum), "--queue-bypass"}})
	}
	return rules, nil
}

func iptsBatchAppendUnique(ipts []*iptables.IPTables, rules []iptRule) error {
	for _, r := range rules {
		for _, ipt := range ipts {
			err := ipt.AppendUnique(r.Table, r.Chain, r.RuleSpec...)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func iptsBatchDeleteIfExists(ipts []*iptables.IPTables, rules []iptRule) error {
	for _, r := range rules {
		for _, ipt := range ipts {
			err := ipt.DeleteIfExists(r.Table, r.Chain, r.RuleSpec...)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
